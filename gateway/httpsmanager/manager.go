// Package httpsmanager owns staged TLS activation independently of the app gateway.
package httpsmanager

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Dir, Host, HTTPSPort, TrialPort, Upstream, Identity string
	Window                                              time.Duration
}
type Revision struct {
	ID          string    `json:"id"`
	Certificate string    `json:"certificate"`
	SealedKey   string    `json:"sealed_key"`
	CA          string    `json:"ca,omitempty"`
	Expires     time.Time `json:"expires"`
	Trust       string    `json:"trust"`
}
type Event struct {
	At       time.Time `json:"at"`
	Action   string    `json:"action"`
	Revision string    `json:"revision,omitempty"`
}
type State struct {
	Active, Previous, Pending string
	Deadline                  time.Time
	Token                     string
	HTTPSRequired             bool
	Revisions                 map[string]Revision
	History                   []Event
}
type Manager struct {
	recoveryKey string
	mu          sync.Mutex
	testing     string
	cfg         Config
	state       State
	seal        cipher.AEAD
	proxy       *httputil.ReverseProxy
	csrf        string
	client      *http.Client
}
type Metadata struct {
	ID      string    `json:"id"`
	Expires time.Time `json:"expires"`
	Trust   string    `json:"trust"`
}

func randomID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func New(cfg Config) (*Manager, error) {
	if cfg.TrialPort == "" {
		cfg.TrialPort = "9443"
	}
	if cfg.Window == 0 {
		cfg.Window = time.Minute
	}
	if cfg.HTTPSPort == "" {
		cfg.HTTPSPort = "443"
	}
	if net.ParseIP(cfg.Host) == nil {
		if len(cfg.Host) == 0 || len(cfg.Host) > 253 {
			return nil, errors.New("configure one hostname or IP address")
		}
		for _, label := range strings.Split(strings.TrimSuffix(cfg.Host, "."), ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return nil, errors.New("invalid HTTPS hostname")
			}
			for _, r := range label {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
					return nil, errors.New("invalid HTTPS hostname")
				}
			}
		}
	}

	if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(cfg.Dir, 0700); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(cfg.Dir, "master.key")
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		err = os.WriteFile(keyPath, key, 0600)
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("invalid HTTPS storage key; preserve the original master.key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	seal, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(cfg.Upstream)
	if err != nil || target.Host == "" {
		return nil, errors.New("invalid gateway upstream")
	}
	m := &Manager{cfg: cfg, seal: seal, csrf: randomID(), client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, proxy: httputil.NewSingleHostReverseProxy(target)}
	director := m.proxy.Director
	m.proxy.Director = func(r *http.Request) {
		secure := r.TLS != nil
		director(r)
		r.Header.Del("X-Forwarded-For")
		r.Header.Set("X-Forwarded-Proto", "http")
		if secure {
			r.Header.Set("X-Forwarded-Proto", "https")
		}
	}
	m.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "The application gateway is unavailable. Local HTTPS recovery remains available.", 503)
	}
	recoveryPath := filepath.Join(cfg.Dir, "recovery.key")
	recovery, readErr := os.ReadFile(recoveryPath)
	if os.IsNotExist(readErr) {
		recovery = []byte(randomID())
		readErr = os.WriteFile(recoveryPath, recovery, 0600)
	}
	if readErr != nil || len(strings.TrimSpace(string(recovery))) != 64 {
		return nil, errors.New("recovery key is unreadable; preserve the original HTTPS state")
	}
	m.recoveryKey = strings.TrimSpace(string(recovery))

	m.state.Revisions = map[string]Revision{}
	data, err := os.ReadFile(filepath.Join(cfg.Dir, "state.json"))
	if err == nil {
		if err = json.Unmarshal(data, &m.state); err != nil {
			stable, e := os.ReadFile(filepath.Join(cfg.Dir, "last-good.json"))
			if e != nil || json.Unmarshal(stable, &m.state) != nil {
				m.state = State{HTTPSRequired: true, Revisions: map[string]Revision{}}
				m.event("state-error-use-local-recovery", "")
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for _, name := range []string{"master.key", "recovery.key", "state.json", "last-good.json"} {
		if err := os.Chmod(filepath.Join(cfg.Dir, name), 0600); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	if m.state.History == nil {
		m.state.History = []Event{}
	}
	if m.state.Revisions == nil {
		m.state.Revisions = map[string]Revision{}
	}
	if m.state.Pending != "" {
		m.event("restart-rollback", m.state.Pending)
		m.state.Pending = ""
		m.state.Token = ""
		m.state.Deadline = time.Time{}
		if err = m.save(); err != nil {
			return nil, err
		}
	}
	return m, nil
}
func (m *Manager) event(action, id string) {
	m.state.History = append(m.state.History, Event{time.Now().UTC(), action, id})
	if len(m.state.History) > 50 {
		m.state.History = m.state.History[len(m.state.History)-50:]
	}
}
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(m.cfg.Dir, "state.json")
	f, err := os.OpenFile(p+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(p+".tmp", p); err != nil {
		return err
	}
	if m.state.Pending == "" {
		backup := filepath.Join(m.cfg.Dir, "last-good.json")
		if err = os.WriteFile(backup+".tmp", data, 0600); err != nil {
			return err
		}
		return os.Rename(backup+".tmp", backup)
	}
	return nil
}
func (m *Manager) privateKey(r Revision) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(r.SealedKey)
	if err != nil || len(raw) < m.seal.NonceSize() {
		return nil, errors.New("saved private key is unreadable")
	}
	return m.seal.Open(nil, raw[:m.seal.NonceSize()], raw[m.seal.NonceSize():], []byte(r.ID))
}
func (m *Manager) pair(r Revision) (tls.Certificate, error) {
	key, err := m.privateKey(r)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair([]byte(r.Certificate), key)
}
func validate(certPEM, keyPEM, caPEM, host string) (tls.Certificate, *x509.Certificate, *x509.CertPool, error) {
	rest := bytes.TrimSpace([]byte(certPEM))
	for len(rest) > 0 {
		block, next := pem.Decode(rest)
		if block == nil || block.Type != "CERTIFICATE" {
			return tls.Certificate{}, nil, nil, errors.New("invalid certificate PEM chain")
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return tls.Certificate{}, nil, nil, errors.New("invalid certificate PEM chain")
		}
		rest = bytes.TrimSpace(next)
	}

	pair, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return pair, nil, nil, errors.New("certificate and private key are invalid or do not match")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return pair, nil, nil, errors.New("invalid leaf certificate")
	}
	if leaf.IsCA {
		return pair, nil, nil, errors.New("upload a server certificate, not a CA certificate and its signing key")
	}
	if key, ok := leaf.PublicKey.(*rsa.PublicKey); ok && key.N.BitLen() < 2048 {
		return pair, nil, nil, errors.New("RSA certificates require keys of at least 2048 bits")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if caPEM != "" {
		roots = x509.NewCertPool()
		rest := []byte(caPEM)
		count := 0
		for len(rest) > 0 {
			b, next := pem.Decode(rest)
			if b == nil {
				return pair, nil, nil, errors.New("invalid private CA bundle")
			}
			rest = next
			c, e := x509.ParseCertificate(b.Bytes)
			if e != nil || !c.IsCA {
				return pair, nil, nil, errors.New("CA bundle must contain CA certificates")
			}
			roots.AddCert(c)
			count++
		}
		if count == 0 {
			return pair, nil, nil, errors.New("empty CA bundle")
		}
	}
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		c, e := x509.ParseCertificate(der)
		if e != nil {
			return pair, nil, nil, errors.New("invalid certificate chain")
		}
		intermediates.AddCert(c)
	}
	if _, err = leaf.Verify(x509.VerifyOptions{DNSName: host, Roots: roots, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		return pair, nil, nil, errors.New("certificate must be current, cover the configured hostname/IP, and chain to a trusted CA (or the supplied private CA)")
	}
	if leaf.NotAfter.Before(time.Now().Add(24 * time.Hour)) {
		return pair, nil, nil, errors.New("certificate expires within 24 hours")
	}
	return pair, leaf, roots, nil
}
func (m *Manager) Stage(cert, key, ca string) (Metadata, error) {
	_, leaf, _, err := validate(cert, key, ca, m.cfg.Host)
	if err != nil {
		return Metadata{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Pending != "" {
		return Metadata{}, errors.New("finish or roll back the pending activation first")
	}
	if len(m.state.Revisions) >= 20 {
		return Metadata{}, errors.New("20 certificate restore points are retained; archive old restore points locally before uploading more")
	}
	id := randomID()
	nonce := make([]byte, m.seal.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return Metadata{}, err
	}
	sealed := m.seal.Seal(nonce, nonce, []byte(key), []byte(id))
	trust := "system"
	if ca != "" {
		trust = "private-ca: browsers and agents must trust this CA"
	}
	rev := Revision{id, cert, base64.StdEncoding.EncodeToString(sealed), ca, leaf.NotAfter, trust}
	m.state.Revisions[id] = rev
	m.event("validated", id)
	if err = m.save(); err != nil {
		delete(m.state.Revisions, id)
		return Metadata{}, err
	}
	return Metadata{id, leaf.NotAfter, trust}, nil
}
func (m *Manager) probe(ctx context.Context, rev Revision) error {
	key, err := m.privateKey(rev)
	if err != nil {
		return err
	}
	pair, _, roots, err := validate(rev.Certificate, string(key), rev.CA, m.cfg.Host)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.New("cannot start temporary HTTPS listener")
	}
	server := &http.Server{Handler: m.proxy, ReadHeaderTimeout: 10 * time.Second}
	go server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}))
	defer server.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: m.cfg.Host}, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, listener.Addr().String())
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	for _, path := range []string{"/", "/api/healthcheck"} {
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://"+net.JoinHostPort(m.cfg.Host, "443")+path, nil)
		resp, e := client.Do(req)
		if e != nil {
			return errors.New("temporary HTTPS handshake or gateway check failed")
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("gateway health check failed (%d); active configuration was preserved", resp.StatusCode)
		}
	}
	return nil
}
func (m *Manager) Activate(ctx context.Context, id string) (string, error) {
	m.mu.Lock()
	if m.state.Pending != "" || m.testing != "" {
		m.mu.Unlock()
		return "", errors.New("an activation is already pending or being checked")
	}
	rev, ok := m.state.Revisions[id]
	if !ok {
		m.mu.Unlock()
		return "", errors.New("candidate not found")
	}
	m.testing = id
	m.mu.Unlock()
	err := m.probe(ctx, rev)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.testing != id {
		return "", errors.New("activation was cancelled")
	}
	m.testing = ""
	if err != nil {
		return "", err
	}
	m.state.Pending = id
	m.state.Token = randomID()
	m.state.Deadline = time.Now().Add(m.cfg.Window)
	m.event("activation-pending", id)
	if err = m.save(); err != nil {
		m.state.Pending = ""
		m.state.Token = ""
		return "", err
	}
	return m.state.Token, nil
}
func (m *Manager) confirm(ctx context.Context, token string) error {
	m.mu.Lock()
	m.rollbackExpired()
	if m.state.Pending == "" || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(m.state.Token)) != 1 {
		m.mu.Unlock()
		return errors.New("activation expired or confirmation is invalid")
	}
	id := m.state.Pending
	rev := m.state.Revisions[id]
	m.mu.Unlock()
	if err := m.probe(ctx, rev); err != nil {
		return errors.New("gateway verification failed; previous configuration remains active")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollbackExpired()
	if m.state.Pending != id || subtle.ConstantTimeCompare([]byte(token), []byte(m.state.Token)) != 1 {
		return errors.New("activation expired or was cancelled")
	}
	old := m.state
	m.state.Previous = m.state.Active
	m.state.Active = id
	m.state.Pending = ""
	m.state.Token = ""
	m.state.Deadline = time.Time{}
	m.state.HTTPSRequired = true
	m.event("confirmed", id)
	if err := m.save(); err != nil {
		m.state = old
		return errors.New("could not persist confirmation")
	}
	return nil
}
func (m *Manager) rollbackExpired() {
	if m.state.Pending != "" && !time.Now().Before(m.state.Deadline) {
		id := m.state.Pending
		m.state.Pending = ""
		m.state.Token = ""
		m.state.Deadline = time.Time{}
		m.event("confirmation-timeout-rollback", id)
		_ = m.save()
	}
}
func (m *Manager) Watch(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.mu.Lock()
			m.rollbackExpired()
			m.mu.Unlock()
		}
	}
}
func (m *Manager) TLSConfig(trial bool) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, SessionTicketsDisabled: trial, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.rollbackExpired()
		id := m.state.Active
		if trial {
			id = m.state.Pending
		}
		r, ok := m.state.Revisions[id]
		if !ok {
			return nil, errors.New("HTTPS is not configured")
		}
		p, e := m.pair(r)
		return &p, e
	}}
}

type trialKey struct{}

func (m *Manager) TrialHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "" {
			w.Header().Set("Connection", "close")
		}
		m.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), trialKey{}, true)))
	})
}
func (m *Manager) secureURL(path string) string {
	host := m.cfg.Host
	if m.cfg.HTTPSPort != "443" {
		host = net.JoinHostPort(host, m.cfg.HTTPSPort)
	}
	return "https://" + host + path
}
func (m *Manager) admin(r *http.Request) bool {
	token := r.Header.Get("Authorization")
	if !strings.HasPrefix(token, "Bearer ") || len(token) > 16384 {
		return false
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", m.cfg.Identity+"/api/auth/user", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", token)
	resp, err := m.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var user struct {
		ID    string `json:"id"`
		Admin bool   `json:"admin"`
	}
	return resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user) == nil && user.ID != "" && user.Admin
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, msg string) {
	reply(w, code, map[string]string{"error": msg})
}
func (m *Manager) API(w http.ResponseWriter, r *http.Request, local bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == "POST" && r.URL.Path == "/api/https/confirm" {
		if r.TLS == nil || r.Context().Value(trialKey{}) != true {
			fail(w, 403, "Confirmation must arrive through the candidate HTTPS listener")
			return
		}
		if err := m.confirm(r.Context(), r.Header.Get("X-Activation-Token")); err != nil {
			fail(w, 409, err.Error())
			return
		}
		reply(w, 200, map[string]bool{"confirmed": true})
		return
	}
	if !local && !m.admin(r) {
		fail(w, 403, "A current Hub instance administrator is required")
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/https/status" {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.rollbackExpired()
		revs := []Metadata{}
		for _, rev := range m.state.Revisions {
			revs = append(revs, Metadata{rev.ID, rev.Expires, rev.Trust})
		}
		reply(w, 200, map[string]any{"host": m.cfg.Host, "https_url": m.secureURL("/"), "active": m.state.Active, "previous": m.state.Previous, "pending": m.state.Pending, "deadline": m.state.Deadline, "history": m.state.History, "certificates": revs, "recovery": "http://localhost:9080", "secure_upload": r.TLS != nil || local})
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "Method not allowed")
		return
	}
	if r.TLS == nil && !local {
		fail(w, 403, "Upload and activate only over trusted HTTPS or server-local recovery (localhost:9080 / SSH tunnel)")
		return
	}
	var input struct {
		ID          string `json:"id"`
		Certificate string `json:"certificate"`
		Key         string `json:"private_key"`
		CA          string `json:"ca"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&input); err != nil {
		fail(w, 400, "Invalid or oversized certificate request")
		return
	}
	switch r.URL.Path {
	case "/api/https/candidates":
		meta, err := m.Stage(input.Certificate, input.Key, input.CA)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		reply(w, 201, meta)
	case "/api/https/activate":
		token, err := m.Activate(r.Context(), input.ID)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		reply(w, 200, map[string]any{"confirmation_url": "https://" + net.JoinHostPort(m.cfg.Host, m.cfg.TrialPort) + "/https-confirm#" + token, "seconds": int(m.cfg.Window.Seconds())})
	case "/api/https/restore":
		m.mu.Lock()
		defer m.mu.Unlock()
		old := m.state
		m.testing = ""
		if m.state.Pending != "" {
			id := m.state.Pending
			m.state.Pending = ""
			m.state.Token = ""
			m.state.Deadline = time.Time{}
			m.event("manual-rollback", id)
		} else {
			rev, ok := m.state.Revisions[m.state.Previous]
			if !ok {
				fail(w, 409, "No previous HTTPS restore point; use local recovery to upload a valid certificate")
				return
			}
			key, err := m.privateKey(rev)
			if err != nil {
				fail(w, 400, "Restore key is unreadable")
				return
			}
			if _, _, _, err = validate(rev.Certificate, string(key), rev.CA, m.cfg.Host); err != nil {
				fail(w, 400, "Previous certificate is no longer valid; use local recovery")
				return
			}
			m.state.Active, m.state.Previous = m.state.Previous, m.state.Active
			m.event("restored", m.state.Active)
		}
		if err := m.save(); err != nil {
			m.state = old
			fail(w, 500, "Could not persist restore")
			return
		}
		reply(w, 200, map[string]bool{"restored": true})
	default:
		fail(w, 404, "Not found")
	}
}
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.rollbackExpired()
	requireTLS := m.state.HTTPSRequired
	m.mu.Unlock()
	if r.TLS == nil && requireTLS {
		http.Redirect(w, r, m.secureURL(r.URL.RequestURI()), 308)
		return
	}

	if r.URL.Path == "/https-confirm" {
		if r.TLS == nil {
			http.Redirect(w, r, m.secureURL("/https-confirm"), 307)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Frame-Options", "DENY")
		io.WriteString(w, strings.ReplaceAll(confirmPage, "DASHBOARD_URL", m.secureURL("/")))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/https/") {
		m.API(w, r, false)
		return
	}
	m.mu.Lock()
	m.rollbackExpired()
	redirect := m.state.HTTPSRequired
	m.mu.Unlock()
	if r.TLS == nil && redirect {
		http.Redirect(w, r, m.secureURL(r.URL.RequestURI()), 308)
		return
	}
	m.proxy.ServeHTTP(w, r)
}
func (m *Manager) Recovery(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		http.Error(w, "Recovery is server-local only", 403)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.URL.Path == "/" && r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, strings.ReplaceAll(recoveryPage, "CSRF_TOKEN", m.csrf))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/https/") {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Recovery-Key")), []byte(m.recoveryKey)) != 1 || r.Header.Get("X-Recovery-Token") != m.csrf || (r.Header.Get("Origin") != "http://"+r.Host && !(r.Method == "GET" && r.Header.Get("Origin") == "")) {
			fail(w, 403, "Open recovery directly on localhost; cross-origin requests are refused")
			return
		}
		m.API(w, r, true)
		return
	}
	http.NotFound(w, r)
}

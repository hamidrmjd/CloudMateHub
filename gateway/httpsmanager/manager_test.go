package httpsmanager

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixture(t *testing.T, host string, expires time.Time) (string, string, string) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Owned test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, _ = x509.ParseCertificate(der)
	leafkey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: time.Now().Add(-time.Hour), NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{host}}
	leafder, err := x509.CreateCertificate(rand.Reader, leaf, root, &leafkey.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	private, _ := x509.MarshalPKCS8PrivateKey(leafkey)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafder})), string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})), string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
func setup(t *testing.T) *Manager {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			c, _, _ := w.(http.Hijacker).Hijack()
			defer c.Close()
			io.WriteString(c, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\nhello")
			return
		}
		if r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("X-Saw-HTTPS", "yes")
		}
		w.WriteHeader(200)
		io.WriteString(w, "ready")
	}))
	t.Cleanup(backend.Close)
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer admin" {
			io.WriteString(w, `{"id":"admin","admin":true}`)
		} else {
			io.WriteString(w, `{"id":"member","admin":false}`)
		}
	}))
	t.Cleanup(identity.Close)
	m, err := New(Config{Dir: t.TempDir(), Host: "hub.example.com", Upstream: backend.URL, Identity: identity.URL, Window: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func stage(t *testing.T, m *Manager) Metadata {
	t.Helper()
	c, k, ca := fixture(t, m.cfg.Host, time.Now().Add(60*24*time.Hour))
	meta, e := m.Stage(c, k, ca)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(filepath.Join(m.cfg.Dir, "state.json"))
	if bytes.Contains(data, []byte("BEGIN PRIVATE KEY")) {
		t.Fatal("plaintext key persisted")
	}
	return meta
}
func confirm(t *testing.T, m *Manager, token string) {
	t.Helper()
	r := httptest.NewRequest("POST", "https://hub.example.com/api/https/confirm", nil)
	r = r.WithContext(context.WithValue(r.Context(), trialKey{}, true))
	r.Header.Set("X-Activation-Token", token)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("confirmation: %d %s", w.Code, w.Body.String())
	}
}
func TestValidationLeavesActiveUntouched(t *testing.T) {
	m := setup(t)
	meta := stage(t, m)
	token, e := m.Activate(t.Context(), meta.ID)
	if e != nil {
		t.Fatal(e)
	}
	confirm(t, m, token)
	for _, kind := range []string{"wrong-host", "expired", "mismatch", "untrusted", "broken-chain"} {
		t.Run(kind, func(t *testing.T) {
			host := m.cfg.Host
			expires := time.Now().Add(60 * 24 * time.Hour)
			if kind == "wrong-host" {
				host = "elsewhere.example.com"
			}
			if kind == "expired" {
				expires = time.Now().Add(-time.Minute)
			}
			c, k, ca := fixture(t, host, expires)
			if kind == "mismatch" {
				_, k, _ = fixture(t, host, expires)
			}
			if kind == "untrusted" {
				ca = ""
			}
			if kind == "broken-chain" {
				c += "\n-----BEGIN CERTIFICATE-----\nbroken\n-----END CERTIFICATE-----"
			}
			if _, e := m.Stage(c, k, ca); e == nil {
				t.Fatal("invalid candidate accepted")
			}
			if m.state.Active != meta.ID {
				t.Fatal("working certificate changed")
			}
		})
	}
}
func TestPendingActivationKeepsProductionAndRollsBack(t *testing.T) {
	m := setup(t)
	first := stage(t, m)
	token, e := m.Activate(t.Context(), first.ID)
	if e != nil {
		t.Fatal(e)
	}
	confirm(t, m, token)
	second := stage(t, m)
	if _, e = m.Activate(t.Context(), second.ID); e != nil {
		t.Fatal(e)
	}
	prod, _ := m.TLSConfig(false).GetCertificate(nil)
	pending, _ := m.TLSConfig(true).GetCertificate(nil)
	if bytes.Equal(prod.Certificate[0], pending.Certificate[0]) {
		t.Fatal("candidate replaced production before confirmation")
	}
	m.state.Deadline = time.Now().Add(-time.Second)
	m.mu.Lock()
	m.rollbackExpired()
	m.mu.Unlock()
	if m.state.Active != first.ID || m.state.Pending != "" {
		t.Fatal("timeout did not restore working state")
	}
	third := stage(t, m)
	if _, e = m.Activate(t.Context(), third.ID); e != nil {
		t.Fatal(e)
	}
	restarted, e := New(m.cfg)
	if e != nil {
		t.Fatal(e)
	}
	if restarted.state.Pending != "" || restarted.state.Active != first.ID {
		t.Fatal("restart did not roll back")
	}
}
func TestHealthFailureCannotActivate(t *testing.T) {
	m := setup(t)
	meta := stage(t, m)
	m.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) { w.WriteHeader(503) }
	m.proxy.Director = func(r *http.Request) { r.URL.Scheme = "http"; r.URL.Host = "127.0.0.1:1" }
	if _, e := m.Activate(t.Context(), meta.ID); e == nil {
		t.Fatal("failed gateway activated")
	}
	if m.state.Active != "" || m.state.Pending != "" {
		t.Fatal("state changed")
	}
}
func TestAPIAdminTransportAndRecoveryBoundaries(t *testing.T) {
	m := setup(t)
	for _, tc := range []struct {
		path, token string
		want        int
	}{{"/api/https/status", "member", 403}, {"/api/https/status", "", 403}, {"/api/https/candidates", "admin", 403}} {
		method := "GET"
		if strings.Contains(tc.path, "candidates") {
			method = "POST"
		}
		r := httptest.NewRequest(method, "http://hub.example.com"+tc.path, strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+tc.token)
		r.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "http://evil.example:9080/", nil)
	w := httptest.NewRecorder()
	m.Recovery(w, r)
	if w.Code != 403 {
		t.Fatal("DNS rebinding allowed")
	}
	r = httptest.NewRequest("POST", "http://localhost:9080/api/https/restore", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("X-Recovery-Token", m.csrf)
	r.Header.Set("X-Recovery-Key", m.recoveryKey)
	w = httptest.NewRecorder()
	m.Recovery(w, r)
	if w.Code != 403 {
		t.Fatal("recovery CSRF allowed")
	}
	r = httptest.NewRequest("GET", "http://localhost:9080/api/https/status", nil)
	r.Header.Set("X-Recovery-Token", m.csrf)
	r.Header.Set("X-Recovery-Key", m.recoveryKey)
	w = httptest.NewRecorder()
	m.Recovery(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(w.Body.String(), "sealed_key") || strings.Contains(w.Body.String(), m.csrf) {
		t.Fatal("status leaked secrets")
	}
}
func TestConfirmationRequiresCandidateTLSAndRestore(t *testing.T) {
	m := setup(t)
	a := stage(t, m)
	token, e := m.Activate(t.Context(), a.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, endpoint := range []string{"http://hub.example.com/api/https/confirm", "https://hub.example.com/api/https/confirm"} {
		r := httptest.NewRequest("POST", endpoint, nil)
		r.Header.Set("X-Activation-Token", token)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("confirmation accepted outside candidate listener")
		}
	}
	confirm(t, m, token)
	b := stage(t, m)
	token, e = m.Activate(t.Context(), b.ID)
	if e != nil {
		t.Fatal(e)
	}
	confirm(t, m, token)
	r := httptest.NewRequest("POST", "https://hub.example.com/api/https/restore", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer admin")
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 200 || m.state.Active != a.ID {
		t.Fatal("restore failed")
	}
	r = httptest.NewRequest("GET", "http://hub.example.com/devices?x=1", nil)
	w = httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 308 || w.Header().Get("Location") != "https://hub.example.com/devices?x=1" {
		t.Fatal("HTTP did not redirect securely")
	}
}
func TestCandidateTLSProxySupportsWebSocketUpgrade(t *testing.T) {
	m := setup(t)
	meta := stage(t, m)
	if _, e := m.Activate(t.Context(), meta.ID); e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: m.TrialHandler()}
	go server.Serve(tls.NewListener(listener, m.TLSConfig(true)))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(m.state.Revisions[meta.ID].CA))
	conn, e := tls.Dial("tcp", listener.Addr().String(), &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: m.cfg.Host})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	io.WriteString(conn, "GET /ws/test HTTP/1.1\r\nHost: hub.example.com\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	reader := bufio.NewReader(conn)
	resp, e := http.ReadResponse(reader, nil)
	if e != nil || resp.StatusCode != 101 {
		t.Fatalf("WebSocket upgrade failed: %v", e)
	}
	b := make([]byte, 5)
	if _, e = io.ReadFull(reader, b); e != nil || string(b) != "hello" {
		t.Fatal("WebSocket bytes not proxied")
	}
}
func TestCorruptStagingRecoversStableSnapshot(t *testing.T) {
	m := setup(t)
	a := stage(t, m)
	token, e := m.Activate(t.Context(), a.ID)
	if e != nil {
		t.Fatal(e)
	}
	confirm(t, m, token)
	if e = os.WriteFile(filepath.Join(m.cfg.Dir, "state.json"), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	restored, e := New(m.cfg)
	if e != nil || restored.state.Active != a.ID {
		t.Fatal("last good snapshot not recovered")
	}
	data, _ := json.Marshal(restored.state.History)
	if len(data) == 0 {
		t.Fatal("history lost")
	}
}

func TestWatchRollsBackWithoutTrafficAndRecoverySurvives(t *testing.T) {
	m := setup(t)
	m.cfg.Window = 20 * time.Millisecond
	candidate := stage(t, m)
	if _, err := m.Activate(t.Context(), candidate.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go m.Watch(ctx)
	time.Sleep(1100 * time.Millisecond)
	m.mu.Lock()
	pending := m.state.Pending
	m.mu.Unlock()
	if pending != "" {
		t.Fatal("watchdog did not clear pending state")
	}
	m.mu.Lock()
	m.state.HTTPSRequired = true
	m.mu.Unlock()
	r := httptest.NewRequest("GET", "http://localhost:9080/", nil)
	w := httptest.NewRecorder()
	m.Recovery(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "HTTPS recovery") {
		t.Fatal("recovery unavailable without a working TLS certificate")
	}
}

func TestCertificateHealthCheckDoesNotBlockCurrentDashboard(t *testing.T) {
	m := setup(t)
	candidate := stage(t, m)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/healthcheck" {
			close(entered)
			<-release
		}
		w.WriteHeader(200)
		io.WriteString(w, "ready")
	}))
	defer func() { unblock(); backend.Close() }()
	target, _ := url.Parse(backend.URL)
	m.proxy = httputil.NewSingleHostReverseProxy(target)
	finished := make(chan error, 1)
	go func() { _, err := m.Activate(t.Context(), candidate.ID); finished <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("candidate probe did not start")
	}
	page := make(chan int, 1)
	go func() {
		r := httptest.NewRequest("GET", "http://hub.example.com/", nil)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		page <- w.Code
	}()
	select {
	case code := <-page:
		if code != 200 {
			t.Fatal("current dashboard unavailable")
		}
	case <-time.After(time.Second):
		t.Fatal("candidate check blocked current dashboard")
	}
	// Cancel the owned request before closing the mocked backend.
	m.mu.Lock()
	m.testing = ""
	m.mu.Unlock()
	unblock()
	if err := <-finished; err == nil {
		t.Fatal("cancelled activation was accepted")
	}
}

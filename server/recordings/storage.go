// Package recordings stores completed SSH recordings outside the session database.
// PostgreSQL remains the durable capture buffer, so unavailable archive storage
// never causes a successful connection's output to be discarded.
package recordings

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shellhub-io/shellhub/pkg/api/query"
	"github.com/shellhub-io/shellhub/pkg/api/scope"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/store"
)

type Config struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Path           string `json:"path,omitempty"`
	Endpoint       string `json:"endpoint,omitempty"`
	Region         string `json:"region,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	Prefix         string `json:"prefix,omitempty"`
	AccessKey      string `json:"access_key,omitempty"`
	SecretKey      string `json:"secret_key,omitempty"`
	SessionToken   string `json:"session_token,omitempty"`
	CredentialsSet bool   `json:"credentials_set"`
}
type state struct {
	Current string            `json:"current"`
	Targets map[string]Config `json:"targets"`
}
type archive struct {
	OutputEvents int64     `json:"output_events"`
	ResizeEvents int64     `json:"resize_events"`
	SeenAt       time.Time `json:"seen_at"`
	Target       string    `json:"target"`
	Key          string    `json:"key"`
	At           time.Time `json:"at"`
}
type Database interface {
	SessionList(context.Context, scope.Scope, ...store.QueryOption) ([]models.Session, int64, error)
	SessionResolve(context.Context, scope.Scope, store.SessionResolver, string, ...store.QueryOption) (*models.Session, error)
	SessionEventsList(context.Context, models.UID, int, models.SessionEventType, ...store.QueryOption) ([]models.SessionEvent, int64, error)
	SessionEventsDelete(context.Context, models.UID, int, models.SessionEventType) error
	Options() store.QueryOptions
}
type Manager struct {
	mu          sync.Mutex
	work        sync.Mutex
	dir         string
	db          Database
	seal        cipher.AEAD
	state       state
	client      *http.Client
	lastError   string
	lastSuccess time.Time
	pending     int
	page        int64
}

func id() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func atomicFile(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".pending-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), name); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func New(dir string, db Database) (*Manager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "index"), 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "probes"), 0700); err != nil {
		return nil, err
	}
	key, err := os.ReadFile(filepath.Join(dir, "master.key"))
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err == nil {
			err = atomicFile(filepath.Join(dir, "master.key"), key)
		}
	}
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	seal, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	m := &Manager{dir: dir, db: db, seal: seal, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	data, err := os.ReadFile(filepath.Join(dir, "storage.enc"))
	if err == nil {
		if len(data) < seal.NonceSize() {
			return nil, errors.New("invalid recording storage state")
		}
		plain, err := seal.Open(nil, data[:seal.NonceSize()], data[seal.NonceSize():], nil)
		if err != nil {
			return nil, errors.New("cannot decrypt recording storage state")
		}
		if err = json.Unmarshal(plain, &m.state); err != nil {
			return nil, err
		}
		if _, ok := m.state.Targets[m.state.Current]; !ok {
			return nil, errors.New("missing current recording destination")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		c := Config{ID: id(), Kind: "local", Path: "/recordings/files"}
		if err = os.MkdirAll(c.Path, 0700); err != nil {
			return nil, err
		}
		m.state = state{Current: c.ID, Targets: map[string]Config{c.ID: c}}
		if err = m.save(m.state); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return m, nil
}
func (m *Manager) save(s state) error {
	plain, err := json.Marshal(s)
	if err != nil {
		return err
	}
	nonce := make([]byte, m.seal.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	return atomicFile(filepath.Join(m.dir, "storage.enc"), m.seal.Seal(nonce, nonce, plain, nil))
}
func redacted(c Config) Config {
	c.CredentialsSet = c.AccessKey != "" && c.SecretKey != ""
	c.AccessKey = ""
	c.SecretKey = ""
	c.SessionToken = ""
	return c
}
func (m *Manager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{"config": redacted(m.state.Targets[m.state.Current]), "recording_required": true, "last_error": m.lastError, "last_success": m.lastSuccess, "pending": m.pending, "capture_buffer": "PostgreSQL", "scope": "Completed session recordings"}
}
func (m *Manager) prepare(c Config) (Config, error) {
	m.mu.Lock()
	old := m.state.Targets[m.state.Current]
	m.mu.Unlock()
	// Blank secrets retain the existing credential only for the same destination.
	if c.Kind == "s3" && c.Kind == old.Kind && c.Endpoint == old.Endpoint && c.Bucket == old.Bucket && c.Region == old.Region && c.AccessKey == "" && c.SecretKey == "" {
		c.AccessKey = old.AccessKey
		c.SecretKey = old.SecretKey
		c.SessionToken = old.SessionToken
	}
	c.ID = id()
	c.CredentialsSet = false
	switch c.Kind {
	case "local", "nfs":
		root := "/recordings"
		if c.Kind == "nfs" {
			root = "/mnt/recordings"
		}
		c.Path = filepath.Clean(c.Path)
		rel, err := filepath.Rel(root, c.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || !filepath.IsAbs(c.Path) {
			return c, fmt.Errorf("choose a folder beneath %s", root)
		}
		if c.Kind == "nfs" && !nfsMounted(root) {
			return c, errors.New("mount an NFS volume at /mnt/recordings before selecting it")
		}
		// Never silently follow a host-mounted symlink outside the storage mount.
		base, err := os.OpenRoot(root)
		if err != nil {
			return c, errors.New("recording mount is unavailable")
		}
		err = base.MkdirAll(rel, 0700)
		base.Close()
		if err != nil {
			return c, errors.New("recording folder is unavailable or escapes its mount")
		}
		real, err := filepath.EvalSymlinks(c.Path)
		if err != nil || !(real == root || strings.HasPrefix(real, root+"/")) {
			return c, errors.New("recording folder escapes its storage mount")
		}
		c = Config{ID: c.ID, Kind: c.Kind, Path: c.Path}
	case "s3":
		if c.Region == "" {
			c.Region = "us-east-1"
		}
		if c.Endpoint == "" {
			c.Endpoint = "https://s3." + c.Region + ".amazonaws.com"
		}
		u, err := url.Parse(c.Endpoint)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
			return c, errors.New("S3 endpoint must be an HTTPS origin without credentials, query or path")
		}
		if c.Bucket == "" || strings.ContainsAny(c.Bucket, "/\\\r\n") || c.AccessKey == "" || c.SecretKey == "" {
			return c, errors.New("S3 bucket, access key and secret key are required")
		}
		if len(c.Prefix) > 512 || strings.Contains(c.Prefix, "..") || strings.ContainsAny(c.Prefix, "\\\r\n") {
			return c, errors.New("invalid S3 prefix")
		}
		c.Prefix = strings.Trim(c.Prefix, "/")
		c.Path = ""
	default:
		return c, errors.New("choose local, nfs or s3 storage")
	}
	return c, nil
}
func nfsMounted(root string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) > 2 && f[1] == root && (f[2] == "nfs" || f[2] == "nfs4") {
			return true
		}
	}
	return false
}
func (m *Manager) Configure(ctx context.Context, c Config, save bool) error {
	// A stalled archive destination must not prevent an admin from selecting a new one.
	c, err := m.prepare(c)
	if err != nil {
		return err
	}
	// Register exact ownership durably before creating a remote probe object.
	probeID := id()
	key := ".cloudmatehub-storage-check/" + probeID
	probeFile := filepath.Join(m.dir, "probes", probeID+".json")
	probe := map[string]any{"target": redacted(c), "key": key, "owned_by": "storage-roundtrip-check", "created_at": time.Now().UTC()}
	owned, _ := json.Marshal(probe)
	if err = atomicFile(probeFile, owned); err != nil {
		return err
	}
	content := []byte("CloudMateHub storage verification\n")
	if err = m.put(ctx, c, key, bytes.NewReader(content)); err != nil {
		return errors.New("storage write failed; settings were not changed")
	}
	got, readErr := m.get(ctx, c, key)
	if readErr == nil {
		defer got.Close()
		b, e := io.ReadAll(io.LimitReader(got, 1024))
		if e != nil || !bytes.Equal(b, content) {
			readErr = errors.New("roundtrip mismatch")
		}
	}
	cleanupErr := m.remove(ctx, c, key)
	if cleanupErr != nil {
		return errors.New("storage probe cleanup failed; exact owned object is recorded in the server storage probe log; settings were not changed")
	}
	if err = os.Remove(probeFile); err != nil {
		return err
	}
	if readErr != nil {
		return errors.New("storage read verification failed; settings were not changed")
	}
	if !save {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	targets := make(map[string]Config, len(m.state.Targets)+1)
	for k, v := range m.state.Targets {
		targets[k] = v
	}
	targets[c.ID] = c
	s := state{Current: c.ID, Targets: targets}
	if err = m.save(s); err != nil {
		return err
	}
	m.state = s
	m.lastError = ""
	return nil
}

// All filesystem access is scoped through os.Root; even symlink changes cannot
// make a recording read, probe or deletion escape its configured directory.
func localRoot(c Config) (*os.Root, error) {
	basePath := "/recordings"
	if c.Kind == "nfs" {
		basePath = "/mnt/recordings"
		if !nfsMounted(basePath) {
			return nil, errors.New("NFS mount unavailable")
		}
	}
	rel, err := filepath.Rel(basePath, c.Path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, errors.New("recording folder escapes its storage mount")
	}
	base, err := os.OpenRoot(basePath)
	if err != nil {
		return nil, err
	}
	defer base.Close()
	return base.OpenRoot(rel)
}

func (m *Manager) put(ctx context.Context, c Config, key string, src io.ReadSeeker) error {
	if c.Kind == "s3" {
		r, err := m.s3(ctx, c, "PUT", key, src)
		if err != nil {
			return err
		}
		defer r.Body.Close()
		if r.StatusCode/100 != 2 {
			return errors.New("S3 write refused")
		}
		return nil
	}
	root, err := localRoot(c)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = root.MkdirAll(filepath.Dir(key), 0700); err != nil {
		return err
	}
	tmp := key + ".pending-" + id()
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, err = io.Copy(f, src)
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	return root.Rename(tmp, key)
}
func (m *Manager) get(ctx context.Context, c Config, key string) (io.ReadCloser, error) {
	if c.Kind == "s3" {
		r, err := m.s3(ctx, c, "GET", key, nil)
		if err != nil {
			return nil, err
		}
		if r.StatusCode != 200 {
			r.Body.Close()
			return nil, errors.New("S3 recording unavailable")
		}
		return r.Body, nil
	}
	root, err := localRoot(c)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(key)
}
func (m *Manager) remove(ctx context.Context, c Config, key string) error {
	if c.Kind == "s3" {
		r, err := m.s3(ctx, c, "DELETE", key, nil)
		if err != nil {
			return err
		}
		defer r.Body.Close()
		if r.StatusCode/100 != 2 {
			return errors.New("S3 deletion refused")
		}
		return nil
	}
	root, err := localRoot(c)
	if err != nil {
		return err
	}
	defer root.Close()
	err = root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func mac(key []byte, s string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(s))
	return h.Sum(nil)
}

// AWS Signature V4 for S3 single-chunk requests. No redirects are followed and
// certificate verification is always enabled, including S3-compatible endpoints.
func (m *Manager) s3(ctx context.Context, c Config, method, key string, body io.ReadSeeker) (*http.Response, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = "/" + c.Bucket + "/" + path.Join(c.Prefix, key)
	u.RawPath = s3URI(u.Path)
	hash := sha256.New()
	if body != nil {
		if _, err = io.Copy(hash, body); err != nil {
			return nil, err
		}
		if _, err = body.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
	}
	payload := hex.EncodeToString(hash.Sum(nil))
	r, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		end, e := body.Seek(0, io.SeekEnd)
		if e != nil {
			return nil, e
		}
		r.ContentLength = end
		body.Seek(0, io.SeekStart)
	}
	signS3(r, c, payload, time.Now().UTC())
	return m.client.Do(r)
}

func s3URI(value string) string {
	const digits = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.ContainsRune("-_.~/", rune(b)) {
			out.WriteByte(b)
		} else {
			out.WriteByte('%')
			out.WriteByte(digits[b>>4])
			out.WriteByte(digits[b&15])
		}
	}
	return out.String()
}

func signS3(r *http.Request, c Config, payload string, now time.Time) {
	date := now.Format("20060102")
	stamp := now.Format("20060102T150405Z")
	r.Header.Set("X-Amz-Date", stamp)
	r.Header.Set("X-Amz-Content-Sha256", payload)
	headers := "host:" + r.URL.Host + "\n"
	signed := "host"
	if value := r.Header.Get("Range"); value != "" {
		headers += "range:" + value + "\n"
		signed += ";range"
	}
	headers += "x-amz-content-sha256:" + payload + "\nx-amz-date:" + stamp + "\n"
	signed += ";x-amz-content-sha256;x-amz-date"
	if c.SessionToken != "" {
		r.Header.Set("X-Amz-Security-Token", c.SessionToken)
		headers += "x-amz-security-token:" + strings.TrimSpace(c.SessionToken) + "\n"
		signed += ";x-amz-security-token"
	}
	canonical := strings.Join([]string{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, headers, signed, payload}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	sc := date + "/" + c.Region + "/s3/aws4_request"
	k := mac([]byte("AWS4"+c.SecretKey), date)
	k = mac(k, c.Region)
	k = mac(k, "s3")
	k = mac(k, "aws4_request")
	signature := hex.EncodeToString(mac(k, "AWS4-HMAC-SHA256\n"+stamp+"\n"+sc+"\n"+hex.EncodeToString(sum[:])))
	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.AccessKey+"/"+sc+", SignedHeaders="+signed+", Signature="+signature)
}
func (m *Manager) marker(uid string, seat int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%d", uid, seat)))
	return filepath.Join(m.dir, "index", hex.EncodeToString(sum[:])+".json")
}
func (m *Manager) archived(uid string, seat int) (archive, Config, error) {
	b, err := os.ReadFile(m.marker(uid, seat))
	if err != nil {
		return archive{}, Config{}, err
	}
	var a archive
	if err = json.Unmarshal(b, &a); err != nil {
		return a, Config{}, err
	}
	m.mu.Lock()
	c, ok := m.state.Targets[a.Target]
	m.mu.Unlock()
	if !ok {
		return a, c, errors.New("recording destination missing")
	}
	return a, c, nil
}

// Cast streams database events into a v2 asciicast file. It includes terminal
// resizes, stdout and stderr; raw keystrokes and file-transfer payloads are not captured.
func (m *Manager) Cast(ctx context.Context, sess *models.Session, seat int) (*os.File, error) {
	f, err := os.CreateTemp(m.dir, "cast-*")
	if err != nil {
		return nil, err
	}
	os.Remove(f.Name())
	enc := json.NewEncoder(f)
	width, height := uint32(80), uint32(24)
	pty, _, err := m.db.SessionEventsList(ctx, models.UID(sess.UID), seat, models.SessionEventTypePtyRequest)
	if err != nil {
		f.Close()
		return nil, err
	}
	if len(pty) > 0 {
		b, _ := json.Marshal(pty[0].Data)
		var p models.SSHPty
		if json.Unmarshal(b, &p) == nil && p.Columns > 0 && p.Rows > 0 {
			width, height = p.Columns, p.Rows
		}
	}
	if err = enc.Encode(map[string]any{"version": 2, "width": width, "height": height, "timestamp": sess.StartedAt.Unix(), "title": "CloudMateHub session " + sess.UID}); err != nil {
		f.Close()
		return nil, err
	}
	// Read output in pages. Resizes are small and merged by timestamp into output.
	sizes, _, err := m.db.SessionEventsList(ctx, models.UID(sess.UID), seat, models.SessionEventTypeWindowChange)
	if err != nil {
		f.Close()
		return nil, err
	}
	si := 0
	resize := func(e models.SessionEvent) error {
		b, _ := json.Marshal(e.Data)
		var d models.SSHWindowChange
		if json.Unmarshal(b, &d) != nil {
			return nil
		}
		return enc.Encode([]any{max(0, e.Timestamp.Sub(sess.StartedAt).Seconds()), "r", fmt.Sprintf("%dx%d", d.Columns, d.Rows)})
	}
	for page := int64(1); ; page++ {
		events, _, e := m.db.SessionEventsList(ctx, models.UID(sess.UID), seat, models.SessionEventTypePtyOutput, m.db.Options().Paginate(&query.Paginator{Page: page, PerPage: 1000}))
		if e != nil {
			err = e
			break
		}
		for _, event := range events {
			for si < len(sizes) && !sizes[si].Timestamp.After(event.Timestamp) {
				if err = resize(sizes[si]); err != nil {
					break
				}
				si++
			}
			if err != nil {
				break
			}
			b, _ := json.Marshal(event.Data)
			var out models.SSHPtyOutput
			if e = json.Unmarshal(b, &out); e != nil {
				err = e
				break
			}
			if err = enc.Encode([]any{max(0, event.Timestamp.Sub(sess.StartedAt).Seconds()), "o", out.Output}); err != nil {
				break
			}
		}
		if err != nil || len(events) < 1000 {
			break
		}
	}
	for err == nil && si < len(sizes) {
		err = resize(sizes[si])
		si++
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// A liveness timeout may later recover. Never treat an earlier snapshot as
// complete after the same connection emits more output or receives a heartbeat.
func (m *Manager) counts(ctx context.Context, uid string, seat int) (int64, int64, error) {
	opts := []store.QueryOption{m.db.Options().Paginate(&query.Paginator{Page: 1, PerPage: 1})}
	_, output, err := m.db.SessionEventsList(ctx, models.UID(uid), seat, models.SessionEventTypePtyOutput, opts...)
	if err != nil {
		return 0, 0, err
	}
	_, resize, err := m.db.SessionEventsList(ctx, models.UID(uid), seat, models.SessionEventTypeWindowChange, opts...)
	return output, resize, err
}
func (m *Manager) current(ctx context.Context, sess *models.Session, seat int, a archive) bool {
	if !a.SeenAt.Equal(sess.LastSeen) {
		return false
	}
	output, resize, err := m.counts(ctx, sess.UID, seat)
	return err == nil && output == a.OutputEvents && resize == a.ResizeEvents
}
func (m *Manager) Read(ctx context.Context, sess *models.Session, seat int) (io.ReadCloser, error) {
	if !sess.Active {
		a, c, err := m.archived(sess.UID, seat)
		if err == nil && m.current(ctx, sess, seat, a) {
			r, e := m.get(ctx, c, a.Key)
			if e == nil {
				return r, nil
			}
		}
	}
	return m.Cast(ctx, sess, seat)
}
func (m *Manager) export(ctx context.Context, sess *models.Session, seat int) error {
	a, prior, archiveErr := m.archived(sess.UID, seat)
	if archiveErr == nil && m.current(ctx, sess, seat, a) {
		return nil
	}
	if archiveErr != nil && !errors.Is(archiveErr, os.ErrNotExist) {
		return archiveErr
	}
	output, resize, err := m.counts(ctx, sess.UID, seat)
	if err != nil {
		return err
	}
	f, err := m.Cast(ctx, sess, seat)
	if err != nil {
		return err
	}
	defer f.Close()
	m.mu.Lock()
	c := m.state.Targets[m.state.Current]
	m.mu.Unlock()
	key := path.Join("sessions", sess.TenantID, sess.UID, fmt.Sprintf("%d.cast", seat))
	// Refresh an existing snapshot in its original destination, avoiding orphaned
	// objects if a timed-out connection later resumes after a storage change.
	if archiveErr == nil {
		c = prior
		key = a.Key
	}
	if err = m.put(ctx, c, key, f); err != nil {
		return errors.New("recording archive write failed; captured output remains in PostgreSQL")
	}
	b, _ := json.Marshal(archive{Target: c.ID, Key: key, At: time.Now().UTC(), SeenAt: sess.LastSeen, OutputEvents: output, ResizeEvents: resize})
	return atomicFile(m.marker(sess.UID, seat), b)
}

func (m *Manager) Sweep(ctx context.Context) error {
	m.work.Lock()
	defer m.work.Unlock()
	m.mu.Lock()
	m.page++
	page := m.page
	m.mu.Unlock()
	list, count, err := m.db.SessionList(ctx, scope.NewUnbounded("archive completed recordings to the instance administrator's configured destination"), m.db.Options().Paginate(&query.Paginator{Page: page, PerPage: 50}))
	if err != nil {
		return err
	}
	if page*50 >= count {
		m.mu.Lock()
		m.page = 0
		m.mu.Unlock()
	}
	pending := 0
	for i := range list {
		s := &list[i]
		if !s.Recorded || s.Active || time.Since(s.LastSeen) < 30*time.Second {
			continue
		}
		for _, seat := range s.Events.Seats {
			if a, _, e := m.archived(s.UID, seat); e == nil && m.current(ctx, s, seat, a) {
				continue
			}
			pending++
			if err = m.export(ctx, s, seat); err != nil {
				m.mu.Lock()
				m.pending = pending
				m.mu.Unlock()
				return err
			}
		}
	}
	m.mu.Lock()
	m.pending = 0
	m.lastSuccess = time.Now().UTC()
	m.lastError = ""
	m.mu.Unlock()
	return nil
}
func (m *Manager) Run(ctx context.Context) {
	timer := time.NewTicker(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			c, cancel := context.WithTimeout(ctx, 2*time.Minute)
			err := m.Sweep(c)
			cancel()
			if err != nil {
				m.mu.Lock()
				m.lastError = "Recording archive unavailable. Captured output is retained in PostgreSQL and will be retried."
				m.mu.Unlock()
			}
		}
	}
}

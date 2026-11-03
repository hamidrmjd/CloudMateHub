package recordings

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/pkg/api/query"
	"github.com/shellhub-io/shellhub/pkg/api/scope"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/shellhub-io/shellhub/server/api/store"
	"github.com/stretchr/testify/require"
)

type fakeOptions struct {
	store.QueryOptions
	page *query.Paginator
}

func (f *fakeOptions) Paginate(p *query.Paginator) store.QueryOption {
	return func(context.Context) error { f.page = p; return nil }
}

type fakeDB struct {
	Database
	sessions []models.Session
	events   []models.SessionEvent
	options  fakeOptions
}

func (f *fakeDB) Options() store.QueryOptions { return &f.options }
func (f *fakeDB) SessionResolve(_ context.Context, sc scope.Scope, _ store.SessionResolver, uid string, _ ...store.QueryOption) (*models.Session, error) {
	for i := range f.sessions {
		s := &f.sessions[i]
		if s.UID == uid {
			if sc.IsBounded() {
				expected, _ := scope.NewBounded(s.TenantID)
				if sc != expected {
					return nil, store.ErrNoDocuments
				}
			}
			copy := *s
			return &copy, nil
		}
	}
	return nil, store.ErrNoDocuments
}
func (f *fakeDB) SessionList(_ context.Context, _ scope.Scope, _ ...store.QueryOption) ([]models.Session, int64, error) {
	return f.sessions, int64(len(f.sessions)), nil
}
func (f *fakeDB) SessionEventsList(ctx context.Context, uid models.UID, seat int, typ models.SessionEventType, opts ...store.QueryOption) ([]models.SessionEvent, int64, error) {
	f.options.page = nil
	for _, o := range opts {
		if o != nil {
			o(ctx)
		}
	}
	all := []models.SessionEvent{}
	for _, e := range f.events {
		if e.Session == string(uid) && e.Seat == seat && e.Type == typ {
			all = append(all, e)
		}
	}
	count := int64(len(all))
	if p := f.options.page; p != nil {
		start := (p.Page - 1) * p.PerPage
		if start >= count {
			return []models.SessionEvent{}, count, nil
		}
		end := min(start+p.PerPage, count)
		all = all[start:end]
	}
	return all, count, nil
}
func (f *fakeDB) SessionEventsDelete(_ context.Context, uid models.UID, seat int, typ models.SessionEventType) error {
	kept := []models.SessionEvent{}
	for _, e := range f.events {
		if e.Session != string(uid) || e.Seat != seat || e.Type != typ {
			kept = append(kept, e)
		}
	}
	f.events = kept
	return nil
}
func (f *fakeDB) SessionUpdate(_ context.Context, _ scope.Scope, s *models.Session) error {
	for i := range f.sessions {
		if f.sessions[i].UID == s.UID {
			f.sessions[i].Recorded = s.Recorded
			return nil
		}
	}
	return store.ErrNoDocuments
}
func fixture() *fakeDB {
	start := time.Now().Add(-2 * time.Minute).UTC()
	s := models.Session{UID: "owned-session", TenantID: "owned-tenant", StartedAt: start, LastSeen: start.Add(time.Second), Recorded: true, Events: models.SessionEvents{Seats: []int{0, 1}, Types: []string{"exec"}}}
	return &fakeDB{sessions: []models.Session{s}, events: []models.SessionEvent{
		{Session: s.UID, Seat: 0, Type: models.SessionEventTypePtyRequest, Timestamp: start, Data: map[string]any{"columns": 100, "rows": 30}},
		{Session: s.UID, Seat: 0, Type: models.SessionEventTypePtyOutput, Timestamp: start.Add(time.Second), Data: map[string]any{"output": "stdout\r\n"}},
		{Session: s.UID, Seat: 0, Type: models.SessionEventTypeWindowChange, Timestamp: start.Add(2 * time.Second), Data: map[string]any{"columns": 120, "rows": 40}},
		{Session: s.UID, Seat: 0, Type: models.SessionEventTypePtyOutput, Timestamp: start.Add(3 * time.Second), Data: map[string]any{"output": "stderr\r\n"}},
		{Session: s.UID, Seat: 1, Type: models.SessionEventTypePtyOutput, Timestamp: start.Add(time.Second), Data: map[string]any{"output": "second channel"}},
	}}
}
func newTestManager(t *testing.T, db *fakeDB) *Manager {
	t.Helper()
	m, err := New(t.TempDir(), db)
	require.NoError(t, err)
	return m
}
func localFixture(t *testing.T) Config {
	t.Helper()
	root := filepath.Join("/recordings", id())
	t.Cleanup(func() { os.RemoveAll(root) })
	return Config{Kind: "local", Path: root}
}
func readAll(t *testing.T, r io.ReadCloser) string {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}
func TestCastAndMultiSeatArchive(t *testing.T) {
	db := fixture()
	m := newTestManager(t, db)
	cfg := localFixture(t)
	require.NoError(t, m.Configure(context.Background(), cfg, true))
	require.NoError(t, m.Sweep(context.Background()))
	for _, seat := range []int{0, 1} {
		a, c, err := m.archived(db.sessions[0].UID, seat)
		require.NoError(t, err)
		r, err := m.get(context.Background(), c, a.Key)
		require.NoError(t, err)
		cast := readAll(t, r)
		if seat == 0 {
			require.Contains(t, cast, `"width":100`)
			require.Contains(t, cast, `[1,"o","stdout\r\n"]`)
			require.Contains(t, cast, `[2,"r","120x40"]`)
			require.Contains(t, cast, `[3,"o","stderr\r\n"]`)
		} else {
			require.Contains(t, cast, "second channel")
		}
	}
	require.Len(t, db.events, 5, "database recovery copy retained")
	require.NoError(t, m.Configure(context.Background(), localFixture(t), true))
	a, c, err := m.archived(db.sessions[0].UID, 0)
	require.NoError(t, err)
	require.Equal(t, cfg.Path, c.Path)
	r, err := m.get(context.Background(), c, a.Key)
	require.NoError(t, err)
	require.Contains(t, readAll(t, r), "stdout")
	again, err := New(m.dir, db)
	require.NoError(t, err)
	_, c, err = again.archived(db.sessions[0].UID, 0)
	require.NoError(t, err)
	require.Equal(t, cfg.Path, c.Path)
}
func TestStorageValidationAndProbeCleanup(t *testing.T) {
	m := newTestManager(t, fixture())
	before := m.Status()["config"].(Config)
	for _, cfg := range []Config{{Kind: "local", Path: "/etc"}, {Kind: "local", Path: "/recordings/../etc"}, {Kind: "nfs", Path: "/mnt/recordings"}, {Kind: "s3", Endpoint: "http://example.com", Bucket: "bucket", AccessKey: "fixture", SecretKey: "fixture"}} {
		require.Error(t, m.Configure(context.Background(), cfg, true))
		require.Equal(t, before, m.Status()["config"])
	}
	cfg := localFixture(t)
	require.NoError(t, m.Configure(context.Background(), cfg, false))
	require.Equal(t, before, m.Status()["config"])
	entries, err := os.ReadDir(filepath.Join(m.dir, "probes"))
	require.NoError(t, err)
	require.Empty(t, entries)
	entries, err = os.ReadDir(filepath.Join(cfg.Path, ".cloudmatehub-storage-check"))
	require.NoError(t, err)
	require.Empty(t, entries)
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(cfg.Path, 0700))
	require.NoError(t, os.Symlink(outside, filepath.Join(cfg.Path, "escape")))
	require.Error(t, m.Configure(context.Background(), Config{Kind: "local", Path: filepath.Join(cfg.Path, "escape")}, true))
}
func TestS3RoundtripSecretsAndFailureFallback(t *testing.T) {
	db := fixture()
	m := newTestManager(t, db)
	objects := map[string][]byte{}
	var mu sync.Mutex
	refuse := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if refuse {
			w.WriteHeader(503)
			return
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=owned-key/") || r.Header.Get("X-Amz-Content-Sha256") == "" {
			w.WriteHeader(403)
			return
		}
		switch r.Method {
		case "PUT":
			b, _ := io.ReadAll(r.Body)
			objects[r.URL.Path] = b
		case "GET":
			b, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(404)
				return
			}
			w.Write(b)
		case "DELETE":
			delete(objects, r.URL.Path)
			w.WriteHeader(204)
		}
	}))
	defer srv.Close()
	m.client = srv.Client()
	cfg := Config{Kind: "s3", Endpoint: srv.URL, Region: "us-east-1", Bucket: "owned-bucket", Prefix: "records", AccessKey: "owned-key", SecretKey: "owned-secret", SessionToken: "owned-token"}
	require.NoError(t, m.Configure(context.Background(), cfg, true))
	require.Empty(t, objects, "roundtrip probe cleaned")
	data, err := os.ReadFile(filepath.Join(m.dir, "storage.enc"))
	require.NoError(t, err)
	require.NotContains(t, string(data), "owned-secret")
	status, _ := json.Marshal(m.Status())
	require.NotContains(t, string(status), "owned-key")
	require.NotContains(t, string(status), "owned-token")
	require.NoError(t, m.Sweep(context.Background()))
	require.Len(t, objects, 2)
	a, c, err := m.archived(db.sessions[0].UID, 0)
	require.NoError(t, err)
	require.Contains(t, a.Key, "owned-session")
	require.Equal(t, "owned-secret", c.SecretKey)
	mu.Lock()
	refuse = true
	mu.Unlock()
	r, err := m.Read(context.Background(), &db.sessions[0], 0)
	require.NoError(t, err)
	require.Contains(t, readAll(t, r), "stdout", "playback falls back to durable capture buffer")
	newer := newTestManager(t, db)
	newer.state = m.state
	newer.client = srv.Client()
	require.Error(t, newer.export(context.Background(), &db.sessions[0], 0))
	require.Len(t, db.events, 5)
}
func TestDeleteOnlyOwnedRecordingAndPreserveHistory(t *testing.T) {
	db := fixture()
	m := newTestManager(t, db)
	require.NoError(t, m.Configure(context.Background(), localFixture(t), true))
	require.NoError(t, m.Sweep(context.Background()))
	require.NoError(t, m.Delete(context.Background(), &db.sessions[0]))
	require.False(t, db.sessions[0].Recorded)
	require.Len(t, db.sessions, 1)
	require.Len(t, db.events, 2)
	_, _, err := m.archived("owned-session", 0)
	require.ErrorIs(t, err, os.ErrNotExist)
}
func TestCastPaginatesOutput(t *testing.T) {
	db := fixture()
	s := db.sessions[0]
	db.events = nil
	for i := 0; i < 1005; i++ {
		db.events = append(db.events, models.SessionEvent{Session: s.UID, Type: models.SessionEventTypePtyOutput, Timestamp: s.StartedAt.Add(time.Duration(i) * time.Millisecond), Data: map[string]any{"output": "line"}})
	}
	m := newTestManager(t, db)
	f, err := m.Cast(context.Background(), &s, 0)
	require.NoError(t, err)
	require.Equal(t, 1005, strings.Count(readAll(t, f), `"line"`))
}
func TestS3SignatureMatchesAWSExample(t *testing.T) {
	r, err := http.NewRequest("GET", "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	require.NoError(t, err)
	r.Header.Set("Range", "bytes=0-9")
	sum := sha256.Sum256(nil)
	signS3(r, Config{AccessKey: "AKIAIOSFODNN7EXAMPLE", SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", Region: "us-east-1"}, hex.EncodeToString(sum[:]), time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	require.Contains(t, r.Header.Get("Authorization"), "Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41")
}
func TestRoutesDenyMembersAndCrossNamespacePlayback(t *testing.T) {
	db := fixture()
	m := newTestManager(t, db)
	router := echo.New()
	router.Use(gateway.WithContext())
	m.Register(router)
	req := func(method, path, body string, headers map[string]string) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w.Code
	}
	require.Equal(t, 403, req("GET", "/api/storage", "", nil))
	require.Equal(t, 403, req("PUT", "/api/storage", `{}`, nil))
	require.Equal(t, 403, req("POST", "/api/storage/test", `{}`, nil))
	require.Equal(t, 200, req("GET", "/api/storage", "", map[string]string{"X-Admin": "true"}))
	require.Equal(t, 404, req("GET", "/api/sessions/owned-session/records/0", "", map[string]string{"X-Tenant-ID": "another-tenant", "X-Role": "owner"}))
	require.Equal(t, 200, req("GET", "/api/sessions/owned-session/records/1", "", map[string]string{"X-Tenant-ID": "owned-tenant", "X-Role": "owner"}))
	require.Equal(t, 404, req("GET", "/api/sessions/owned-session/records/99", "", map[string]string{"X-Tenant-ID": "owned-tenant", "X-Role": "owner"}))
	require.Equal(t, 400, req("PUT", "/api/storage", `{"kind":"s3","access_key":"fixture","secret_key":"fixture"}`, map[string]string{"X-Admin": "true"}))
	db.sessions[0].Active = true
	require.Equal(t, 409, req("DELETE", "/api/sessions/owned-session/records/0", "", map[string]string{"X-Tenant-ID": "owned-tenant", "X-Role": "owner"}))
}

func TestAdminCanChangeStorageWhileArchivingIsStalled(t *testing.T) {
	m := newTestManager(t, fixture())
	cfg := localFixture(t)
	m.work.Lock()
	done := make(chan error, 1)
	go func() { done <- m.Configure(context.Background(), cfg, true) }()
	select {
	case err := <-done:
		m.work.Unlock()
		require.NoError(t, err)
	case <-time.After(time.Second):
		m.work.Unlock()
		<-done
		t.Fatal("storage configuration was blocked by archive work")
	}
}

func TestChangingFolderToSymlinkCannotEscapeStorageMount(t *testing.T) {
	m := newTestManager(t, fixture())
	cfg := localFixture(t)
	owned := cfg.Path
	cfg.Path = filepath.Join(owned, "selected")
	require.NoError(t, m.Configure(context.Background(), cfg, true))
	require.NoError(t, os.Rename(cfg.Path, filepath.Join(owned, "moved")))
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, cfg.Path))
	require.Error(t, m.put(context.Background(), cfg, "owned-probe", bytes.NewReader([]byte("fixture"))))
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
	// Validation must not create subfolders through an escaping symlink either.
	cfg.Path = filepath.Join(cfg.Path, "new-folder")
	require.Error(t, m.Configure(context.Background(), cfg, true))
	entries, err = os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestLateOutputRefreshesArchiveWithoutServingStaleRecording(t *testing.T) {
	db := fixture()
	m := newTestManager(t, db)
	cfg := localFixture(t)
	require.NoError(t, m.Configure(context.Background(), cfg, true))
	require.NoError(t, m.Sweep(context.Background()))
	db.events = append(db.events, models.SessionEvent{Session: db.sessions[0].UID, Seat: 0, Type: models.SessionEventTypePtyOutput, Timestamp: db.sessions[0].StartedAt.Add(4 * time.Second), Data: map[string]any{"output": "late output"}})
	r, err := m.Read(context.Background(), &db.sessions[0], 0)
	require.NoError(t, err)
	require.Contains(t, readAll(t, r), "late output", "playback must fall back to the complete database capture")
	require.NoError(t, m.Configure(context.Background(), localFixture(t), true))
	require.NoError(t, m.Sweep(context.Background()))
	a, c, err := m.archived(db.sessions[0].UID, 0)
	require.NoError(t, err)
	require.Equal(t, cfg.Path, c.Path)
	r, err = m.get(context.Background(), c, a.Key)
	require.NoError(t, err)
	require.Contains(t, readAll(t, r), "late output")
}

// Package directory connects Hub accounts to Active Directory and OpenLDAP.
package directory

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-ldap/ldap/v3"
	"github.com/shellhub-io/shellhub/pkg/cache"
	"github.com/shellhub-io/shellhub/server/api/store"
)

type Config struct {
	Kind              string `json:"kind"`
	ID                string `json:"id"`
	Label             string `json:"label"`
	Enabled           bool   `json:"enabled"`
	URL               string `json:"url"`
	BaseDN            string `json:"base_dn"`
	BindDN            string `json:"bind_dn"`
	BindPassword      string `json:"bind_password,omitempty"`
	PasswordSet       bool   `json:"password_set"`
	CA                string `json:"ca_pem"`
	Filter            string `json:"user_filter"`
	UsernameAttribute string `json:"username_attribute"`
	IDAttribute       string `json:"id_attribute"`
	EmailAttribute    string `json:"email_attribute"`
	NameAttribute     string `json:"name_attribute"`
	RequiredGroup     string `json:"required_group_dn"`
	Provision         bool   `json:"provision_accounts"`
}
type ldapConnection interface {
	Bind(string, string) error
	Search(*ldap.SearchRequest) (*ldap.SearchResult, error)
	Close() error
}
type Manager struct {
	mu       sync.RWMutex
	loginMu  sync.Mutex
	dir      string
	seal     cipher.AEAD
	profiles map[string]Config
	revision map[string]string
	db       store.Store
	cache    cache.Cache
	connect  func(context.Context, Config) (ldapConnection, error)
}

var ErrDenied = errors.New("directory authentication failed")

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func defaults(kind string) Config {
	c := Config{Kind: kind, ID: newID(), UsernameAttribute: "uid", IDAttribute: "entryUUID", EmailAttribute: "mail", NameAttribute: "cn", Filter: "(&(objectClass=inetOrgPerson)(uid={username}))", Label: "OpenLDAP"}
	if kind == "ad" {
		c.Label = "Active Directory"
		c.UsernameAttribute = "sAMAccountName"
		c.IDAttribute = "objectGUID"
		c.EmailAttribute = "userPrincipalName"
		c.NameAttribute = "displayName"
		c.Filter = "(&(objectCategory=person)(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(|(sAMAccountName={username})(userPrincipalName={username})))"
	}
	return c
}
func writeFile(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".directory-*")
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
	e := f.Close()
	if err != nil {
		return err
	}
	if e != nil {
		return e
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
func New(dir string, db store.Store, c cache.Cache) (*Manager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	key, err := os.ReadFile(filepath.Join(dir, "master.key"))
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err == nil {
			err = writeFile(filepath.Join(dir, "master.key"), key)
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
	m := &Manager{dir: dir, seal: seal, db: db, cache: c, profiles: map[string]Config{}, revision: map[string]string{}}
	m.connect = dial
	raw, err := os.ReadFile(filepath.Join(dir, "directories.enc"))
	if err == nil {
		if len(raw) < seal.NonceSize() {
			return nil, errors.New("invalid directory settings")
		}
		plain, e := seal.Open(nil, raw[:seal.NonceSize()], raw[seal.NonceSize():], nil)
		if e != nil {
			return nil, errors.New("cannot decrypt directory settings")
		}
		if err = json.Unmarshal(plain, &m.profiles); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, kind := range []string{"ad", "openldap"} {
		if _, ok := m.profiles[kind]; !ok {
			m.profiles[kind] = defaults(kind)
		}
		m.revision[kind] = newID()
	}
	if err = m.save(m.profiles); err != nil {
		return nil, err
	}
	return m, nil
}
func (m *Manager) save(profiles map[string]Config) error {
	b, err := json.Marshal(profiles)
	if err != nil {
		return err
	}
	nonce := make([]byte, m.seal.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	return writeFile(filepath.Join(m.dir, "directories.enc"), m.seal.Seal(nonce, nonce, b, nil))
}
func redacted(c Config) Config { c.PasswordSet = c.BindPassword != ""; c.BindPassword = ""; return c }
func (m *Manager) Settings() []Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return []Config{redacted(m.profiles["ad"]), redacted(m.profiles["openldap"])}
}
func (m *Manager) Public() []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := []map[string]string{}
	for _, k := range []string{"ad", "openldap"} {
		c := m.profiles[k]
		if c.Enabled {
			result = append(result, map[string]string{"kind": c.Kind, "label": c.Label})
		}
	}
	return result
}

var attribute = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9;-]{0,63}$`)

func validate(c Config) error {
	if c.Kind != "ad" && c.Kind != "openldap" {
		return errors.New("choose Active Directory or OpenLDAP")
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "ldaps" && u.Scheme != "ldap") || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("use an ldaps:// server or ldap:// server with mandatory StartTLS")
	}
	if _, err = ldap.ParseDN(c.BaseDN); err != nil || c.BaseDN == "" {
		return errors.New("a valid base DN is required")
	}
	if c.BindDN == "" || c.BindPassword == "" {
		return errors.New("a read-only bind account and password are required")
	}
	if c.Label == "" || len(c.Label) > 64 || len(c.BindPassword) > 4096 || len(c.CA) > 65536 || len(c.Filter) > 4096 {
		return errors.New("invalid directory settings size")
	}
	if strings.Count(c.Filter, "{username}") < 1 || strings.Count(c.Filter, "{username}") > 4 {
		return errors.New("user filter must contain {username}")
	}
	if _, err = ldap.CompileFilter(strings.ReplaceAll(c.Filter, "{username}", "fixture")); err != nil {
		return errors.New("invalid LDAP user filter")
	}
	for _, a := range []string{c.UsernameAttribute, c.IDAttribute, c.EmailAttribute, c.NameAttribute} {
		if !attribute.MatchString(a) {
			return errors.New("invalid LDAP attribute name")
		}
	}
	if c.RequiredGroup != "" {
		if _, err = ldap.ParseDN(c.RequiredGroup); err != nil {
			return errors.New("invalid group DN")
		}
	}
	if c.CA != "" {
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(c.CA)) {
			return errors.New("invalid CA certificate PEM")
		}
	}
	return nil
}
func (m *Manager) prepare(c Config) (Config, error) {
	m.mu.RLock()
	old, ok := m.profiles[c.Kind]
	m.mu.RUnlock()
	if !ok {
		return c, errors.New("unknown directory type")
	}
	c.ID = old.ID
	c.PasswordSet = false
	same := c.URL == old.URL && c.BaseDN == old.BaseDN && c.IDAttribute == old.IDAttribute
	if same && c.BindDN == old.BindDN && c.BindPassword == "" {
		c.BindPassword = old.BindPassword
	}
	if !same {
		c.ID = newID()
	}
	return c, validate(c)
}
func (m *Manager) Configure(ctx context.Context, c Config) error {
	// Disabling only changes the enabled flag; it cannot overwrite saved secrets
	// or change the directory identity through a partially filled form.
	if !c.Enabled {
		m.mu.Lock()
		defer m.mu.Unlock()
		old, ok := m.profiles[c.Kind]
		if !ok {
			return errors.New("unknown directory type")
		}
		old.Enabled = false
		next := map[string]Config{}
		for k, v := range m.profiles {
			next[k] = v
		}
		next[c.Kind] = old
		if err := m.save(next); err != nil {
			return err
		}
		m.profiles = next
		m.revision[c.Kind] = newID()
		return nil
	}
	c, err := m.prepare(c)
	if err != nil {
		return err
	}
	if err = m.Test(ctx, c); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := map[string]Config{}
	for k, v := range m.profiles {
		next[k] = v
	}
	next[c.Kind] = c
	if err = m.save(next); err != nil {
		return err
	}
	m.profiles = next
	m.revision[c.Kind] = newID()
	return nil
}
func (m *Manager) Test(ctx context.Context, c Config) error {
	if err := validate(c); err != nil {
		return err
	}
	conn, err := m.connect(ctx, c)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err = conn.Bind(c.BindDN, c.BindPassword); err != nil {
		return errors.New("directory bind failed; check the read-only service account")
	}
	r, err := conn.Search(ldap.NewSearchRequest(c.BaseDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 5, false, "(objectClass=*)", []string{"dn"}, nil))
	if err != nil || r == nil || len(r.Entries) != 1 {
		return errors.New("base DN is unavailable to the bind account")
	}
	return nil
}

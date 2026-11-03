package directory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/shellhub-io/shellhub/pkg/cache"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/store"
	"github.com/stretchr/testify/require"
)

type fixtureDB struct {
	store.Store
	users   map[string]*models.User
	creates int
	fail    error
}

func (d *fixtureDB) UserResolve(_ context.Context, r store.UserResolver, v string, _ ...store.QueryOption) (*models.User, error) {
	if d.fail != nil {
		return nil, d.fail
	}
	for _, u := range d.users {
		if r == store.UserExternalIDResolver && u.ExternalID == v || r == store.UserEmailResolver && u.Email == v || r == store.UserIDResolver && u.ID == v {
			return u, nil
		}
	}
	return nil, store.ErrNoDocuments
}
func (d *fixtureDB) UserCreate(_ context.Context, u *models.User) (string, error) {
	d.creates++
	u.ID = "fixture-user"
	d.users[u.ID] = u
	return u.ID, nil
}

type fakeLDAP struct {
	binds     []string
	filters   []string
	ambiguous bool
	deny      bool
	group     bool
	stable    string
	searchErr error
}

func (f *fakeLDAP) Bind(dn, password string) error {
	f.binds = append(f.binds, dn)
	if password == "bad" {
		return errors.New("secret must not be exposed")
	}
	return nil
}
func (*fakeLDAP) Close() error { return nil }
func (f *fakeLDAP) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	f.filters = append(f.filters, req.Filter)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if req.Scope == ldap.ScopeBaseObject {
		if f.deny {
			return &ldap.SearchResult{}, nil
		}
		return &ldap.SearchResult{Entries: []*ldap.Entry{{DN: req.BaseDN}}}, nil
	}
	e := ldap.NewEntry("uid=alice,dc=example,dc=com", map[string][]string{"uid": {"alice"}, "entryUUID": {f.stable}, "mail": {"alice@example.com"}, "cn": {"Alice"}, "sAMAccountName": {"alice"}, "objectGUID": {f.stable}, "userPrincipalName": {"alice@example.com"}, "displayName": {"Alice"}})
	entries := []*ldap.Entry{e}
	if f.ambiguous {
		entries = append(entries, e)
	}
	return &ldap.SearchResult{Entries: entries}, nil
}
func fixture(t *testing.T) (*Manager, *fixtureDB, *fakeLDAP, Config) {
	t.Helper()
	d := &fixtureDB{users: map[string]*models.User{}}
	m, err := New(t.TempDir(), d, cache.NewNullCache())
	require.NoError(t, err)
	f := &fakeLDAP{stable: "fixture-stable-id"}
	m.connect = func(context.Context, Config) (ldapConnection, error) { return f, nil }
	c := m.profiles["openldap"]
	c.URL = "ldaps://directory.example.com:636"
	c.BaseDN = "dc=example,dc=com"
	c.BindDN = "cn=reader,dc=example,dc=com"
	c.BindPassword = "fixture-secret"
	c.Enabled = true
	c.Provision = true
	require.NoError(t, m.Configure(context.Background(), c))
	return m, d, f, m.profiles["openldap"]
}
func TestEncryptedSettingsAndRedaction(t *testing.T) {
	m, _, _, c := fixture(t)
	raw, err := os.ReadFile(filepath.Join(m.dir, "directories.enc"))
	require.NoError(t, err)
	require.NotContains(t, string(raw), c.BindPassword)
	require.NotContains(t, string(raw), c.BindDN)
	for _, c := range m.Settings() {
		require.Empty(t, c.BindPassword)
	}
	info, err := os.Stat(filepath.Join(m.dir, "master.key"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	again, err := New(m.dir, m.db, m.cache)
	require.NoError(t, err)
	require.Equal(t, c.ID, again.profiles[c.Kind].ID)
	require.Equal(t, c.BindPassword, again.profiles[c.Kind].BindPassword)
}
func TestLoginCreatesLeastPrivilegeAndReusesStableIdentity(t *testing.T) {
	m, d, _, c := fixture(t)
	u, err := m.Login(context.Background(), c.Kind, "alice", "valid")
	require.NoError(t, err)
	require.False(t, u.Admin)
	require.Zero(t, u.MaxNamespaces)
	require.Empty(t, u.Password.Hash)
	require.Equal(t, []models.UserAuthMethod{models.UserAuthMethodLDAP}, u.Preferences.AuthMethods)
	require.Equal(t, models.UserOriginLDAP, u.Origin)
	again, err := m.Login(context.Background(), c.Kind, "renamed", "valid")
	require.NoError(t, err)
	require.Equal(t, u.ID, again.ID)
	require.Equal(t, 1, d.creates)
}
func TestEmailCollisionCannotTakeOverLocalAccount(t *testing.T) {
	m, d, _, c := fixture(t)
	d.users["local-admin"] = &models.User{ID: "local-admin", Origin: models.UserOriginLocal, Admin: true, UserData: models.UserData{Email: "alice@example.com"}}
	_, err := m.Login(context.Background(), c.Kind, "alice", "valid")
	require.ErrorIs(t, err, ErrDenied)
	require.Zero(t, d.creates)
	require.True(t, d.users["local-admin"].Admin)
}
func TestNoProvisioningBeforeCredentialProof(t *testing.T) {
	for _, scenario := range []string{"password", "ambiguous", "provision-disabled", "db-unavailable", "group"} {
		t.Run(scenario, func(t *testing.T) {
			m, d, f, c := fixture(t)
			password := "valid"
			switch scenario {
			case "password":
				password = "bad"
			case "ambiguous":
				f.ambiguous = true
			case "provision-disabled":
				c.Provision = false
				m.profiles[c.Kind] = c
			case "db-unavailable":
				d.fail = errors.New("database unavailable")
			case "group":
				c.RequiredGroup = "cn=allowed,dc=example,dc=com"
				m.profiles[c.Kind] = c
				f.deny = true
			}
			_, err := m.Login(context.Background(), c.Kind, "alice", password)
			require.Error(t, err)
			require.Zero(t, d.creates)
		})
	}
}
func TestDisableAndProfileChangeInvalidateSessions(t *testing.T) {
	m, _, _, c := fixture(t)
	u, err := m.Login(context.Background(), c.Kind, "alice", "valid")
	require.NoError(t, err)
	require.NoError(t, m.ValidateUser(context.Background(), u))
	require.NoError(t, m.VerifyUser(context.Background(), u, "valid"))
	require.Error(t, m.VerifyUser(context.Background(), u, "bad"))
	require.NoError(t, m.Configure(context.Background(), Config{Kind: c.Kind}))
	require.Error(t, m.ValidateUser(context.Background(), u))
	c.URL = "ldaps://another.example.com"
	require.NoError(t, m.Configure(context.Background(), c))
	require.Error(t, m.ValidateUser(context.Background(), u))
}
func TestFilterEscapingAndADRestrictions(t *testing.T) {
	c := defaults("ad")
	filter := userFilter(c, `DOMAIN\alice*)(|(objectClass=*))`)
	require.Contains(t, filter, `alice\2a\29\28|\28objectClass=\2a\29\29`)
	require.NotContains(t, filter, "DOMAIN")
	require.Contains(t, filter, "userAccountControl:1.2.840.113556.1.4.803:=2")
	c.RequiredGroup = "CN=Allowed,DC=example,DC=com"
	require.Contains(t, groupFilter(c, filter), "memberOf:1.2.840.113556.1.4.1941:=")
	_, err := ldap.CompileFilter(groupFilter(c, filter))
	require.NoError(t, err)
	require.Contains(t, stableFilter(c, []byte{0, 42, 40, 41}), `\00\2a\28\29`)
}
func TestConfigValidationAndIdentityBinding(t *testing.T) {
	m, _, _, c := fixture(t)
	for _, url := range []string{"http://directory.example.com", "ldaps://user:password@directory.example.com", "ldap://directory.example.com/path"} {
		bad := c
		bad.URL = url
		require.Error(t, validate(bad))
	}
	bad := c
	bad.CA = "invalid"
	require.Error(t, validate(bad))
	bad = c
	bad.Filter = "(uid=*)"
	require.Error(t, validate(bad))
	next := c
	next.BindPassword = ""
	prepared, err := m.prepare(next)
	require.NoError(t, err)
	require.Equal(t, c.BindPassword, prepared.BindPassword)
	require.Equal(t, c.ID, prepared.ID)
	next.URL = "ldaps://other.example.com"
	_, err = m.prepare(next)
	require.Error(t, err)
	next.BindPassword = "new-secret"
	prepared, err = m.prepare(next)
	require.NoError(t, err)
	require.NotEqual(t, c.ID, prepared.ID)
}
func TestAccountNamesFitLocalLimits(t *testing.T) {
	for _, kind := range []string{"ad", "openldap"} {
		name := accountName(defaults(kind), person{Stable: []byte("id"), Login: strings.Repeat("a", 100)})
		require.LessOrEqual(t, len(name), 32)
		require.True(t, strings.HasPrefix(name, kind+"."))
	}
}

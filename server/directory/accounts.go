package directory

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/store"
)

func origin(kind string) models.UserOrigin {
	if kind == "ad" {
		return models.UserOriginActiveDirectory
	}
	return models.UserOriginLDAP
}
func IsDirectory(u *models.User) bool {
	return u.Origin == models.UserOriginActiveDirectory || u.Origin == models.UserOriginLDAP
}
func (m *Manager) profile(kind string) (Config, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profiles[kind], m.revision[kind]
}
func (m *Manager) unchanged(c Config, revision string) bool {
	current, r := m.profile(c.Kind)
	return current.Enabled && current.ID == c.ID && r == revision
}
func eligible(u *models.User, c Config) bool {
	return u.Origin == origin(c.Kind) && u.Status == models.UserStatusConfirmed && !u.AwaitingApproval && slices.Contains(u.Preferences.AuthMethods, models.UserAuthMethodLDAP)
}
func accountName(c Config, p person) string {
	sum := sha256.Sum256([]byte(externalID(c, p.Stable)))
	var name strings.Builder
	for _, r := range strings.ToLower(p.Login) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			name.WriteRune(r)
		}
	}
	login := name.String()
	limit := 32 - len(c.Kind) - 14
	if len(login) > limit {
		login = login[:limit]
	}
	return c.Kind + "." + login + "_" + hex.EncodeToString(sum[:6])
}

// Login never links an account by its email or mutable login name. Only the
// directory's immutable object identifier can select an existing Hub account.
func (m *Manager) Login(ctx context.Context, kind, username, password string) (*models.User, error) {
	c, revision := m.profile(kind)
	if !c.Enabled {
		return nil, ErrDenied
	}
	p, err := m.authenticate(ctx, c, username, password)
	if err != nil {
		return nil, ErrDenied
	}
	m.loginMu.Lock()
	defer m.loginMu.Unlock()
	if !m.unchanged(c, revision) {
		return nil, ErrDenied
	}
	id := externalID(c, p.Stable)
	u, err := m.db.UserResolve(ctx, store.UserExternalIDResolver, id)
	if errors.Is(err, store.ErrNoDocuments) && c.Provision {
		// An email collision is an administrator decision, never an automatic merge.
		if _, e := m.db.UserResolve(ctx, store.UserEmailResolver, p.Email); !errors.Is(e, store.ErrNoDocuments) {
			return nil, ErrDenied
		}
		u = &models.User{Origin: origin(kind), ExternalID: id, Status: models.UserStatusConfirmed, UserData: models.UserData{Name: p.Name, Username: accountName(c, p), Email: p.Email}, Preferences: models.UserPreferences{AuthMethods: []models.UserAuthMethod{models.UserAuthMethodLDAP}}}
		u.ID, err = m.db.UserCreate(ctx, u)
		if err != nil {
			u, err = m.db.UserResolve(ctx, store.UserExternalIDResolver, id)
		}
	}
	if err != nil || u == nil || !eligible(u, c) || !m.unchanged(c, revision) {
		return nil, ErrDenied
	}
	// Do not overwrite the full account with directory metadata during sign-in.
	return u, nil
}
func (m *Manager) userProfile(u *models.User) (Config, string, []byte, error) {
	kind := "openldap"
	if u.Origin == models.UserOriginActiveDirectory {
		kind = "ad"
	}
	c, r := m.profile(kind)
	prefix := "directory:" + c.ID + ":"
	if !IsDirectory(u) || !c.Enabled || !eligible(u, c) || !strings.HasPrefix(u.ExternalID, prefix) {
		return c, r, nil, ErrDenied
	}
	stable, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(u.ExternalID, prefix))
	if err != nil || len(stable) == 0 || len(stable) > 256 {
		return c, r, nil, ErrDenied
	}
	return c, r, stable, nil
}
func stableFilter(c Config, stable []byte) string {
	return "(&" + strings.ReplaceAll(c.Filter, "{username}", "*") + "(" + c.IDAttribute + "=" + ldap.EscapeFilter(string(stable)) + "))"
}
func (m *Manager) VerifyUser(ctx context.Context, u *models.User, password string) error {
	if end, _, err := m.cache.HasAccountLockout(ctx, "directory-stepup", u.ID); err != nil || end > 0 {
		return ErrDenied
	}
	err := m.verifyPassword(ctx, u, password)
	if err != nil {
		_, _, _ = m.cache.StoreLoginAttempt(ctx, "directory-stepup", u.ID)
	} else {
		_ = m.cache.ResetLoginAttempts(ctx, "directory-stepup", u.ID)
	}
	return err
}
func (m *Manager) verifyPassword(ctx context.Context, u *models.User, password string) error {
	c, r, stable, err := m.userProfile(u)
	if err != nil || password == "" || len(password) > 4096 {
		return ErrDenied
	}
	conn, err := m.connect(ctx, c)
	if err != nil {
		return ErrDenied
	}
	defer conn.Close()
	if conn.Bind(c.BindDN, c.BindPassword) != nil {
		return ErrDenied
	}
	p, err := lookup(conn, c, stableFilter(c, stable))
	if err != nil || conn.Bind(p.DN, password) != nil || !m.unchanged(c, r) {
		return ErrDenied
	}
	return nil
}
func (m *Manager) ValidateUser(ctx context.Context, u *models.User) error {
	c, r, stable, err := m.userProfile(u)
	if err != nil {
		return ErrDenied
	}
	digest := sha256.Sum256([]byte(u.ExternalID))
	key := "directory-valid:" + r + ":" + hex.EncodeToString(digest[:])
	var cached bool
	if m.cache.Get(ctx, key, &cached) == nil && cached {
		return nil
	}
	conn, err := m.connect(ctx, c)
	if err != nil {
		return ErrDenied
	}
	defer conn.Close()
	if conn.Bind(c.BindDN, c.BindPassword) != nil {
		return ErrDenied
	}
	if _, err = lookup(conn, c, stableFilter(c, stable)); err != nil || !m.unchanged(c, r) {
		return ErrDenied
	}
	_ = m.cache.Set(ctx, key, true, time.Minute)
	return nil
}

// InWebsiteGroup resolves membership using the authenticated account's stable
// directory identity and the currently enabled directory connection.
func (m *Manager) InWebsiteGroup(ctx context.Context, u *models.User, kind, groupDN string) bool {
	c, revision, stable, err := m.userProfile(u)
	if err != nil || c.Kind != kind {
		return false
	}
	conn, err := m.connect(ctx, c)
	if err != nil {
		return false
	}
	defer conn.Close()
	if conn.Bind(c.BindDN, c.BindPassword) != nil {
		return false
	}
	filter := stableFilter(c, stable)
	if kind == "ad" {
		filter = "(&" + filter + "(memberOf:1.2.840.113556.1.4.1941:=" + ldap.EscapeFilter(groupDN) + "))"
	}
	p, err := lookup(conn, c, filter)
	if err != nil {
		return false
	}
	if kind == "openldap" && !isGroupMember(conn, groupDN, p.DN, p.Login) {
		return false
	}
	return m.unchanged(c, revision)
}

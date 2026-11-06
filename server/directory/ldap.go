package directory

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

type person struct {
	Stable []byte
	Login  string
	Email  string
	Name   string
	DN     string
}

func tlsConfig(c Config) (*tls.Config, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if c.CA != "" && !roots.AppendCertsFromPEM([]byte(c.CA)) {
		return nil, errors.New("invalid CA PEM")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname(), RootCAs: roots}, nil
}
func dial(ctx context.Context, c Config) (ldapConnection, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, errors.New("invalid directory URL")
	}
	cfg, err := tlsConfig(c)
	if err != nil {
		return nil, err
	}
	port := u.Port()
	if port == "" {
		port = "636"
		if u.Scheme == "ldap" {
			port = "389"
		}
	}
	d := net.Dialer{Timeout: 8 * time.Second}
	raw, err := d.DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return nil, errors.New("cannot reach directory server")
	}
	deadline := time.Now().Add(12 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	raw.SetDeadline(deadline)
	secure := u.Scheme == "ldaps"
	if secure {
		t := tls.Client(raw, cfg)
		if err = t.HandshakeContext(ctx); err != nil {
			raw.Close()
			return nil, errors.New("directory TLS validation failed; check hostname and CA trust")
		}
		raw = t
	}
	conn := ldap.NewConn(raw, secure)
	conn.SetTimeout(8 * time.Second)
	conn.Start()
	if !secure {
		if err = conn.StartTLS(cfg); err != nil {
			conn.Close()
			return nil, errors.New("directory StartTLS failed; no credentials were sent")
		}
	}
	return conn, nil
}
func userFilter(c Config, username string) string {
	if c.Kind == "ad" && strings.Contains(username, "\\") {
		_, username, _ = strings.Cut(username, "\\")
	}
	return strings.ReplaceAll(c.Filter, "{username}", ldap.EscapeFilter(username))
}
func groupFilter(c Config, filter string) string {
	if c.Kind == "ad" {
		filter = "(&" + filter + "(!(userAccountControl:1.2.840.113556.1.4.803:=2)))"
		if c.RequiredGroup != "" {
			filter = "(&" + filter + "(memberOf:1.2.840.113556.1.4.1941:=" + ldap.EscapeFilter(c.RequiredGroup) + "))"
		}
	}
	return filter
}

func isGroupMember(conn ldapConnection, groupDN, userDN, username string) bool {
	dn := ldap.EscapeFilter(userDN)
	login := ldap.EscapeFilter(username)
	group, err := conn.Search(ldap.NewSearchRequest(groupDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 5, false, "(|(member="+dn+")(uniqueMember="+dn+")(memberUid="+login+"))", []string{"dn"}, nil))
	return err == nil && group != nil && len(group.Entries) == 1
}

func lookup(conn ldapConnection, c Config, filter string) (person, error) {
	r, err := conn.Search(ldap.NewSearchRequest(c.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 5, false, groupFilter(c, filter), []string{c.IDAttribute, c.UsernameAttribute, c.EmailAttribute, c.NameAttribute}, nil))
	if err != nil || r == nil || len(r.Entries) != 1 {
		return person{}, ErrDenied
	}
	e := r.Entries[0]
	if c.Kind == "openldap" && c.RequiredGroup != "" && !isGroupMember(conn, c.RequiredGroup, e.DN, e.GetAttributeValue(c.UsernameAttribute)) {
		return person{}, ErrDenied
	}
	p := person{Stable: e.GetRawAttributeValue(c.IDAttribute), Login: e.GetAttributeValue(c.UsernameAttribute), Email: strings.ToLower(e.GetAttributeValue(c.EmailAttribute)), Name: e.GetAttributeValue(c.NameAttribute), DN: e.DN}
	address, err := mail.ParseAddress(p.Email)
	if len(p.Stable) == 0 || len(p.Stable) > 256 || p.Login == "" || p.DN == "" || err != nil || address.Address != p.Email {
		return person{}, ErrDenied
	}
	if p.Name == "" {
		p.Name = p.Login
	}
	if len(p.Name) > 64 {
		p.Name = string([]rune(p.Name)[:min(16, len([]rune(p.Name)))])
	}
	return p, nil
}
func (m *Manager) authenticate(ctx context.Context, c Config, username, password string) (person, error) {
	if username == "" || len(username) > 320 || password == "" || len(password) > 4096 {
		return person{}, ErrDenied
	}
	conn, err := m.connect(ctx, c)
	if err != nil {
		return person{}, ErrDenied
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if err = conn.Bind(c.BindDN, c.BindPassword); err != nil {
		return person{}, ErrDenied
	}
	p, err := lookup(conn, c, userFilter(c, username))
	if err != nil {
		return person{}, err
	}
	if err = conn.Bind(p.DN, password); err != nil {
		return person{}, ErrDenied
	}
	return p, nil
}
func externalID(c Config, stable []byte) string {
	return "directory:" + c.ID + ":" + base64.RawURLEncoding.EncodeToString(stable)
}

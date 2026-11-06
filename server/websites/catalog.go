// Package websites manages the instance's assigned website links.
package websites

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/shellhub-io/shellhub/server/api/store"
)

const Path = "/api/assigned-websites"

type Assignment struct {
	Kind string `json:"kind"`
	Value string `json:"value"`
}

type Website struct {
	ID string `json:"id"`
	Name string `json:"name"`
	URL string `json:"url"`
	Assignments []Assignment `json:"assignments,omitempty"`
}

type identities interface {
	NamespaceResolve(context.Context, store.NamespaceResolver, string) (*models.Namespace, error)
	UserResolve(context.Context, store.UserResolver, string, ...store.QueryOption) (*models.User, error)
}

type groupCheck func(context.Context, *models.User, string, string) bool

type Catalog struct {
	mu sync.RWMutex
	path string
	sites []Website
	db identities
	inGroup groupCheck
}

func New(dir string, db identities, inGroup groupCheck) (*Catalog, error) {
	c := &Catalog{path: filepath.Join(dir, "websites.json"), db: db, inGroup: inGroup}
	data, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &c.sites); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) save(sites []Website) error {
	f, err := os.CreateTemp(filepath.Dir(c.path), ".websites-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(sites)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), c.path); err != nil {
		return err
	}
	c.sites = sites
	return nil
}

func valid(site *Website) bool {
	site.Name = strings.TrimSpace(site.Name)
	site.URL = strings.TrimSpace(site.URL)
	u, err := url.Parse(site.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") ||
		strings.ContainsAny(site.URL, "\\\t\r\n") || len(site.URL) > 2048 ||
		site.Name == "" || len(site.Name) > 120 || len(site.Assignments) == 0 || len(site.Assignments) > 32 {
		return false
	}
	for i := range site.Assignments {
		a := &site.Assignments[i]
		a.Value = strings.TrimSpace(a.Value)
		if a.Value == "" || len(a.Value) > 512 {
			return false
		}
		switch a.Kind {
		case "user", "namespace":
		case "ad", "openldap":
			if _, err := ldap.ParseDN(a.Value); err != nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (c *Catalog) list(g *gateway.Context, id string) []Website {
	c.mu.RLock()
	sites := slices.Clone(c.sites)
	c.mu.RUnlock()
	out := make([]Website, 0, len(sites))
	ctx, cancel := context.WithTimeout(g.Ctx(), 5*time.Second)
	defer cancel()
	var user *models.User
	userResolved := false
	checks := make(map[Assignment]bool)
	for _, site := range sites {
		if g.IsAdmin() {
			out = append(out, site)
			continue
		}
		for _, a := range site.Assignments {
			allowed, seen := checks[a]
			if !seen {
				switch a.Kind {
				case "user":
					allowed = a.Value == id
				case "namespace":
					ns, err := c.db.NamespaceResolve(ctx, store.NamespaceTenantIDResolver, a.Value)
					if err == nil {
						_, allowed = ns.FindMember(id)
						allowed = allowed || ns.Owner == id
					}
				case "ad", "openldap":
					if !userResolved {
						user, _ = c.db.UserResolve(ctx, store.UserIDResolver, id)
						userResolved = true
					}
					allowed = ctx.Err() == nil && user != nil && c.inGroup(ctx, user, a.Kind, a.Value)
				}
				checks[a] = allowed
			}
			if allowed {
				site.Assignments = nil
				out = append(out, site)
				break
			}
		}
	}
	return out
}

func (c *Catalog) change(g *gateway.Context) error {
	var site Website
	method := g.Request().Method
	if method != http.MethodDelete {
		body := http.MaxBytesReader(g.Response(), g.Request().Body, 32*1024)
		if json.NewDecoder(body).Decode(&site) != nil || !valid(&site) {
			return g.JSON(http.StatusBadRequest, map[string]string{"error": "Enter a valid HTTP or HTTPS URL, name, and at least one assignment."})
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	sites := slices.Clone(c.sites)
	if method == http.MethodPost {
		if len(sites) >= 100 {
			return g.JSON(http.StatusConflict, map[string]string{"error": "The website catalog has reached its 100 entry limit."})
		}
		var key [16]byte
		if _, err := rand.Read(key[:]); err != nil {
			return g.NoContent(http.StatusInternalServerError)
		}
		site.ID = hex.EncodeToString(key[:])
		sites = append(sites, site)
	} else {
		site.ID = g.Param("id")
		i := slices.IndexFunc(sites, func(s Website) bool { return s.ID == site.ID })
		if i < 0 {
			return g.NoContent(http.StatusNotFound)
		}
		if method == http.MethodDelete {
			sites = slices.Delete(sites, i, i+1)
		} else {
			sites[i] = site
		}
	}
	if err := c.save(sites); err != nil {
		return g.NoContent(http.StatusInternalServerError)
	}
	return g.NoContent(http.StatusNoContent)
}

func (c *Catalog) Register(r *echo.Echo) {
	r.GET(Path, gateway.Handler(func(g *gateway.Context) error {
		id, ok := g.GetID()
		if !ok || g.APIKeyID() != "" {
			return g.NoContent(http.StatusForbidden)
		}
		g.Response().Header().Set("Cache-Control", "no-store")
		return g.JSON(http.StatusOK, c.list(g, id))
	}))
	mutate := gateway.Handler(func(g *gateway.Context) error {
		if _, ok := g.GetID(); !ok || !g.IsAdmin() || g.APIKeyID() != "" {
			return g.NoContent(http.StatusForbidden)
		}
		return c.change(g)
	})
	r.POST(Path, mutate)
	r.PUT(Path+"/:id", mutate)
	r.DELETE(Path+"/:id", mutate)
}

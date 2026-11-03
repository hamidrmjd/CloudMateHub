package directory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/pkg/api/query"
	"github.com/shellhub-io/shellhub/pkg/api/requests"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/pkg/uuid"
	"github.com/shellhub-io/shellhub/pkg/validator"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/shellhub-io/shellhub/server/api/store"
)

type tokenIssuer interface {
	CreateUserToken(context.Context, *requests.CreateUserToken) (*models.UserAuthResponse, error)
}

func decode(c *gateway.Context, value any) error {
	dec := json.NewDecoder(io.LimitReader(c.Request().Body, 100001))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("invalid request")
	}
	return nil
}
func secure(c *gateway.Context) bool {
	return c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https"
}
func (m *Manager) Register(router *echo.Echo, tokens tokenIssuer) {
	router.GET("/api/auth/directories", gateway.Handler(func(c *gateway.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, m.Public())
	}))
	router.POST("/api/auth/directory", gateway.Handler(func(c *gateway.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		if !secure(c) {
			return c.JSON(400, map[string]string{"error": "Trusted HTTPS is required for directory sign-in"})
		}
		var req struct {
			Kind     string `json:"kind"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if decode(c, &req) != nil {
			return c.NoContent(400)
		}
		// Limit failures by both source address and identifier. Changing usernames
		// does not evade the source limit, and failures never enumerate Hub users.
		hash := sha256.Sum256([]byte(req.Kind + ":" + strings.ToLower(req.Username)))
		identifier := "directory:" + hex.EncodeToString(hash[:])
		source := c.RealIP()
		for _, key := range []string{"directory-source", identifier} {
			if end, _, err := m.cache.HasAccountLockout(c.Ctx(), source, key); err != nil {
				return c.NoContent(503)
			} else if end > 0 {
				c.Response().Header().Set("X-Account-Lockout", strconv.FormatInt(end, 10))
				return c.NoContent(429)
			}
		}
		user, err := m.Login(c.Ctx(), req.Kind, req.Username, req.Password)
		if err != nil {
			for _, key := range []string{"directory-source", identifier} {
				_, _, _ = m.cache.StoreLoginAttempt(c.Ctx(), source, key)
			}
			return c.JSON(401, map[string]string{"error": "Directory sign-in failed. Check your credentials or contact your administrator."})
		}
		_ = m.cache.ResetLoginAttempts(c.Ctx(), source, identifier)
		if user.MFA.Enabled {
			token := uuid.Generate()
			if err = m.cache.Set(c.Ctx(), "mfa-signin={"+token+"}", &models.PendingMFASignIn{UserID: user.ID, TokenVersion: user.TokenVersion}, 30*time.Minute); err != nil {
				return c.NoContent(503)
			}
			c.Response().Header().Set("X-MFA-Token", token)
			return c.NoContent(401)
		}
		result, err := tokens.CreateUserToken(c.Ctx(), &requests.CreateUserToken{UserID: user.ID})
		if err != nil {
			return c.NoContent(503)
		}
		return c.JSON(200, result)
	}))
	router.GET("/api/directories", gateway.Handler(func(c *gateway.Context) error {
		if !c.IsAdmin() {
			return c.NoContent(403)
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, m.Settings())
	}))
	configure := func(test bool) echo.HandlerFunc {
		return gateway.Handler(func(c *gateway.Context) error {
			if !c.IsAdmin() {
				return c.NoContent(403)
			}
			var cfg Config
			if decode(c, &cfg) != nil {
				return c.NoContent(400)
			}
			cfg.Kind = c.Param("kind")
			if (test || cfg.Enabled || cfg.BindPassword != "") && !secure(c) {
				return c.JSON(400, map[string]string{"error": "Enable trusted HTTPS before testing or saving directory credentials"})
			}
			var err error
			if test {
				cfg, err = m.prepare(cfg)
				if err == nil {
					err = m.Test(c.Ctx(), cfg)
				}
			} else {
				err = m.Configure(c.Ctx(), cfg)
			}
			if err != nil {
				return c.JSON(400, map[string]string{"error": err.Error()})
			}
			return c.JSON(200, map[string]bool{"ok": true})
		})
	}
	router.PUT("/api/directories/:kind", configure(false))
	router.POST("/api/directories/:kind/test", configure(true))
	// This proof is deliberately dry-run: it cannot create or modify a Hub user.
	router.POST("/api/directories/:kind/test-login", gateway.Handler(func(c *gateway.Context) error {
		if !c.IsAdmin() {
			return c.NoContent(403)
		}
		if !secure(c) {
			return c.NoContent(400)
		}
		var req struct {
			Config   Config `json:"config"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if decode(c, &req) != nil {
			return c.NoContent(400)
		}
		req.Config.Kind = c.Param("kind")
		cfg, err := m.prepare(req.Config)
		if err == nil {
			_, err = m.authenticate(c.Ctx(), cfg, req.Username, req.Password)
		}
		if err != nil {
			return c.JSON(400, map[string]string{"error": "Directory sign-in check failed; check credentials, attributes and group access"})
		}
		return c.JSON(200, map[string]bool{"ok": true})
	}))
	m.registerUsers(router)
}

type userSummary struct {
	ID               string            `json:"id"`
	Username         string            `json:"username"`
	Name             string            `json:"name"`
	Email            string            `json:"email"`
	Origin           models.UserOrigin `json:"origin"`
	Admin            bool              `json:"admin"`
	Status           models.UserStatus `json:"status"`
	AwaitingApproval bool              `json:"awaiting_approval"`
}

func (m *Manager) registerUsers(router *echo.Echo) {
	router.GET("/api/user-management/users", gateway.Handler(func(c *gateway.Context) error {
		if !c.IsAdmin() {
			return c.NoContent(403)
		}
		page, _ := strconv.ParseInt(c.QueryParam("page"), 10, 64)
		p := &query.Paginator{Page: page, PerPage: 25}
		p.Normalize()
		opts := []store.QueryOption{m.db.Options().Paginate(p), m.db.Options().Sort(&query.Sorter{By: "username", Order: query.OrderAsc})}
		search := strings.TrimSpace(c.QueryParam("search"))
		if len(search) > 255 {
			return c.NoContent(400)
		}
		if search != "" {
			opts = append(opts, m.db.Options().Match(&query.Filters{Data: []query.Filter{{Type: query.FilterTypeProperty, Params: &query.FilterProperty{Name: "username", Operator: "contains", Value: search}}}}))
		}
		users, total, err := m.db.UserList(c.Ctx(), opts...)
		if err != nil {
			return c.NoContent(503)
		}
		result := []userSummary{}
		for _, u := range users {
			result = append(result, userSummary{u.ID, u.Username, u.Name, u.Email, u.Origin, u.Admin, u.Status, u.AwaitingApproval})
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"users": result, "total": total, "page": p.Page, "per_page": p.PerPage})
	}))
	router.POST("/api/user-management/users", gateway.Handler(func(c *gateway.Context) error {
		if !c.IsAdmin() {
			return c.NoContent(403)
		}
		if !secure(c) {
			return c.JSON(400, map[string]string{"error": "Enable trusted HTTPS before creating accounts"})
		}
		var req struct {
			Name     string `json:"name"`
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if decode(c, &req) != nil {
			return c.NoContent(400)
		}
		data := models.UserData{Name: req.Name, Username: strings.ToLower(req.Username), Email: strings.ToLower(req.Email)}
		v := validator.New()
		if ok, _ := v.Struct(data); !ok {
			return c.JSON(400, map[string]string{"error": "Use a name of 1–64 characters, a username of 3–32 lowercase letters, numbers or -_.@, and a valid email"})
		}
		if ok, _ := v.Var(req.Password, "required,min=12,max=32"); !ok {
			return c.JSON(400, map[string]string{"error": "Use a password between 12 and 32 characters"})
		}
		password, err := models.HashUserPassword(req.Password)
		if err != nil {
			return c.NoContent(500)
		}
		u := &models.User{Origin: models.UserOriginLocal, Status: models.UserStatusConfirmed, UserData: data, Password: password, Preferences: models.UserPreferences{AuthMethods: []models.UserAuthMethod{models.UserAuthMethodLocal}}}
		id, err := m.db.UserCreate(c.Ctx(), u)
		if errors.Is(err, store.ErrDuplicate) {
			return c.JSON(409, map[string]string{"error": "Username or email is already in use"})
		}
		if err != nil {
			return c.NoContent(503)
		}
		return c.JSON(http.StatusCreated, map[string]string{"id": id})
	}))
}

package recordings

import (
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/pkg/api/authorizer"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/shellhub-io/shellhub/server/api/store"
)

// Register inherits the global live-identity authenticator. Settings require an
// instance administrator; playback/deletion additionally stay namespace scoped.
func (m *Manager) Register(router *echo.Echo) {
	router.GET("/api/storage", gateway.Handler(func(c *gateway.Context) error {
		if !c.IsAdmin() {
			return c.NoContent(403)
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, m.Status())
	}))
	configure := func(save bool) echo.HandlerFunc {
		return gateway.Handler(func(c *gateway.Context) error {
			if !c.IsAdmin() {
				return c.NoContent(403)
			}
			var cfg Config
			if err := c.Bind(&cfg); err != nil {
				return c.JSON(400, map[string]string{"error": "Invalid storage settings"})
			}
			// Prevent credentials travelling over public plaintext connections. Local
			// reverse proxy transport is HTTP, so use the edge's overwritten scheme.
			if cfg.Kind == "s3" && (cfg.AccessKey != "" || cfg.SecretKey != "" || cfg.SessionToken != "") && c.Request().TLS == nil && c.Request().Header.Get("X-Forwarded-Proto") != "https" {
				return c.JSON(400, map[string]string{"error": "Enable trusted HTTPS before submitting storage credentials"})
			}
			if err := m.Configure(c.Ctx(), cfg, save); err != nil {
				return c.JSON(400, map[string]string{"error": err.Error()})
			}
			return c.JSON(200, map[string]bool{"ok": true})
		})
	}
	router.POST("/api/storage/test", configure(false))
	router.PUT("/api/storage", configure(true))
	route := "/api/sessions/:uid/records/:seat"
	resolve := func(c *gateway.Context, p authorizer.Permission) (*models.Session, int, error) {
		if !c.Role().HasPermission(p) {
			return nil, 0, echo.NewHTTPError(403, "Recording access denied")
		}
		sc, err := c.Scope()
		if err != nil {
			return nil, 0, err
		}
		seat, err := strconv.Atoi(c.Param("seat"))
		if err != nil || seat < 0 || seat > 65535 {
			return nil, 0, echo.NewHTTPError(400, "Invalid recording channel")
		}
		s, err := m.db.SessionResolve(c.Ctx(), sc, store.SessionUIDResolver, c.Param("uid"))
		if err != nil {
			return nil, 0, echo.NewHTTPError(404, "Recording not found")
		}
		found := false
		for _, n := range s.Events.Seats {
			if n == seat {
				found = true
				break
			}
		}
		if !s.Recorded || !found {
			return nil, 0, echo.NewHTTPError(404, "Recording not found")
		}
		return s, seat, nil
	}
	router.GET(route, gateway.Handler(func(c *gateway.Context) error {
		s, seat, err := resolve(c, authorizer.SessionPlay)
		if err != nil {
			return err
		}
		reader, err := m.Read(c.Ctx(), s, seat)
		if err != nil {
			return c.JSON(503, map[string]string{"error": "Recording is temporarily unavailable"})
		}
		defer reader.Close()
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
		c.Response().WriteHeader(http.StatusOK)
		_, err = io.Copy(c.Response(), reader)
		return err
	}))
	// Existing UI deletion means all seats of the session, preserving its history.
	router.DELETE(route, gateway.Handler(func(c *gateway.Context) error {
		s, _, err := resolve(c, authorizer.SessionRemove)
		if err != nil {
			return err
		}
		if s.Active {
			return c.JSON(409, map[string]string{"error": "Close the session before removing its recording"})
		}
		if err = m.Delete(c.Ctx(), s); err != nil {
			return c.JSON(503, map[string]string{"error": "Recording could not be fully removed; retry when storage is available"})
		}
		return c.JSON(200, map[string]bool{"ok": true})
	}))
}

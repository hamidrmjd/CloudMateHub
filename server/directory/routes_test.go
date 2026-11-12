package directory

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/server/api/pkg/echo/handlers"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/shellhub-io/shellhub/server/api/routes/middleware"
	"github.com/stretchr/testify/require"
)

func TestRoutesRejectForgedAdminAndAnonymousSecrets(t *testing.T) {
	m, _, _, _ := fixture(t)
	router := echo.New()
	router.HTTPErrorHandler = handlers.NewErrors(nil)
	auth := middleware.NewAuthenticator(nil)
	router.Use(gateway.WithContext(), auth.Middleware)
	m.Register(router, nil)
	auth.AllowAnonymous("GET", "/api/auth/directories")
	auth.AllowAnonymous("POST", "/api/auth/directory")
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/api/directories", 401}, {"PUT", "/api/directories/ad", 401}, {"POST", "/api/directories/ad/test", 401}, {"POST", "/api/directories/ad/test-login", 401}, {"GET", "/api/user-management/users", 401}, {"GET", "/api/user-management/workspaces", 401}, {"GET", "/api/user-management/workspaces/00000000-0000-4000-8000-000000000000/members", 401}, {"POST", "/api/user-management/users", 401}, {"POST", "/api/auth/directory", 400}, {"GET", "/api/auth/directories", 200}} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			req.Header.Set("X-Admin", "true")
			req.Header.Set("X-ID", "forged")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, tc.status, rec.Code)
			require.NotContains(t, rec.Body.String(), "fixture-secret")
			require.NotContains(t, rec.Body.String(), "cn=reader")
		})
	}
}

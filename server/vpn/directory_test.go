package vpn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/pkg/api/query"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/shellhub-io/shellhub/server/api/store"
	"github.com/shellhub-io/shellhub/server/api/store/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testRouter(t *testing.T) (*echo.Echo, *mocks.MockStore) {
	users := mocks.NewMockStore(t)
	router := echo.New()
	router.Use(gateway.WithContext())
	// This handler-layer test supplies the canonical identity after authentication.
	// Live authentication and forged-header rejection belong to the existing global middleware.
	RegisterDirectory(router, users)
	return router, users
}

func TestDirectoryRequiresInstanceAdminBeforeStoreAccess(t *testing.T) {
	router, _ := testRouter(t)
	for _, path := range []string{"/internal/vpn/users", "/internal/vpn/users/user-id"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusForbidden, response.Code)
	}
}

func TestDirectoryReturnsOnlySafeUserSummary(t *testing.T) {
	router, users := testRouter(t)
	options := mocks.NewMockQueryOptions(t)
	users.On("Options").Return(options).Once()
	options.On("Paginate", &query.Paginator{Page: 1, PerPage: 50}).Return(store.QueryOption(nil)).Once()
	users.On("UserList", mock.Anything, mock.Anything).Return([]models.User{{ID: "existing-user", Admin: true, UserData: models.UserData{Name: "Fixture", Username: "fixture", Email: "private@example.invalid"}}}, int64(1), nil).Once()
	request := httptest.NewRequest(http.MethodGet, "/internal/vpn/users", nil)
	request.Header.Set("X-Admin", "true")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `[{"id":"existing-user","username":"fixture","name":"Fixture"}]`, response.Body.String())
}

func TestDirectoryResolvesOnlyExactID(t *testing.T) {
	router, users := testRouter(t)
	users.On("UserResolve", mock.Anything, store.UserIDResolver, "exact-id").Return(&models.User{ID: "exact-id", UserData: models.UserData{Username: "fixture"}}, nil).Once()
	request := httptest.NewRequest(http.MethodGet, "/internal/vpn/users/exact-id", nil)
	request.Header.Set("X-Admin", "true")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"id":"exact-id","username":"fixture","name":""}`, response.Body.String())
}

func TestDirectoryMissingUserIs404(t *testing.T) {
	router, users := testRouter(t)
	users.On("UserResolve", mock.Anything, store.UserIDResolver, "missing").Return(nil, store.ErrNoDocuments).Once()
	request := httptest.NewRequest(http.MethodGet, "/internal/vpn/users/missing", nil)
	request.Header.Set("X-Admin", "true")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
}

// Package vpn provides the minimal identity directory needed by the VPN broker.
// It is not the upstream Enterprise administration API.
package vpn

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/pkg/api/query"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/shellhub-io/shellhub/server/api/store"
)

type DirectoryStore interface {
	UserList(context.Context, ...store.QueryOption) ([]models.User, int64, error)
	UserResolve(context.Context, store.UserResolver, string, ...store.QueryOption) (*models.User, error)
	Options() store.QueryOptions
}

type userSummary struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

func summary(user models.User) userSummary { return userSummary{user.ID, user.Username, user.Name} }

// RegisterDirectory inherits the router's global authenticator. /internal is
// refused by the public Caddy gateway; the broker calls it on the private network.
func RegisterDirectory(router *echo.Echo, users DirectoryStore) {
	router.GET("/internal/vpn/users", gateway.Handler(func(c *gateway.Context) error {
		if !c.IsAdmin() {
			return c.NoContent(http.StatusForbidden)
		}
		page, _ := strconv.ParseInt(c.QueryParam("page"), 10, 64)
		paginator := &query.Paginator{Page: page, PerPage: 50}
		paginator.Normalize()
		opts := []store.QueryOption{users.Options().Paginate(paginator)}
		search := strings.TrimSpace(c.QueryParam("search"))
		if len(search) > 255 {
			return c.NoContent(http.StatusBadRequest)
		}
		if search != "" {
			opts = append(opts, users.Options().Match(&query.Filters{Data: []query.Filter{{Type: query.FilterTypeProperty, Params: &query.FilterProperty{Name: "username", Operator: "contains", Value: search}}}}))
		}
		list, _, err := users.UserList(c.Ctx(), opts...)
		if err != nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		result := make([]userSummary, 0, len(list))
		for _, user := range list {
			result = append(result, summary(user))
		}
		return c.JSON(http.StatusOK, result)
	}))
	router.GET("/internal/vpn/users/:id", gateway.Handler(func(c *gateway.Context) error {
		if !c.IsAdmin() {
			return c.NoContent(http.StatusForbidden)
		}
		id := c.Param("id")
		if id == "" || len(id) > 255 {
			return c.NoContent(http.StatusBadRequest)
		}
		user, err := users.UserResolve(c.Ctx(), store.UserIDResolver, id)
		if errors.Is(err, store.ErrNoDocuments) {
			return c.NoContent(http.StatusNotFound)
		}
		if err != nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		return c.JSON(http.StatusOK, summary(*user))
	}))
}

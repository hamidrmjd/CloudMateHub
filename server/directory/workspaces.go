package directory

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/pkg/api/query"
	"github.com/shellhub-io/shellhub/pkg/api/scope"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/pkg/uuid"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
)

type workspaceSummary struct {
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
}

func (m *Manager) registerWorkspaces(router *echo.Echo) {
	admin := func(handler func(*gateway.Context) error) echo.HandlerFunc {
		return gateway.Handler(func(c *gateway.Context) error {
			if _, ok := c.GetID(); !ok || !c.IsAdmin() || c.APIKeyID() != "" {
				return c.NoContent(http.StatusForbidden)
			}
			c.Response().Header().Set("Cache-Control", "no-store")
			return handler(c)
		})
	}
	router.GET("/api/user-management/workspaces", admin(func(c *gateway.Context) error {
		page := workspacePage(c)
		opts := m.db.Options()
		list, total, err := m.db.NamespaceList(c.Ctx(), opts.Sort(&query.Sorter{By: "name", Order: query.OrderAsc}), opts.Paginate(page))
		if err != nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		workspaces := make([]workspaceSummary, 0, len(list))
		for _, workspace := range list {
			workspaces = append(workspaces, workspaceSummary{Name: workspace.Name, TenantID: workspace.TenantID})
		}
		return c.JSON(http.StatusOK, map[string]any{"items": workspaces, "total": total, "per_page": page.PerPage})
	}))
	router.GET("/api/user-management/workspaces/:tenant/members", admin(func(c *gateway.Context) error {
		tenant := c.Param("tenant")
		if _, err := uuid.Parse(tenant); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		sc := scope.MustBounded(tenant)
		page := workspacePage(c)
		members, total, err := m.db.NamespaceGetMembers(c.Ctx(), sc, m.db.Options().Paginate(page))
		if err != nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		if members == nil {
			members = []models.MemberView{}
		}
		return c.JSON(http.StatusOK, map[string]any{"items": members, "total": total, "per_page": page.PerPage})
	}))
}

func workspacePage(c *gateway.Context) *query.Paginator {
	page, _ := strconv.ParseInt(c.QueryParam("page"), 10, 32)
	paginator := &query.Paginator{Page: page, PerPage: 25}
	paginator.Normalize()
	return paginator
}

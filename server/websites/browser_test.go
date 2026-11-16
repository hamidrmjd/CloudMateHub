package websites

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/stretchr/testify/require"
)

func TestBrowserChecksEveryEventAndIsolatesSessions(t *testing.T) {
	keys := []string{}
	catalog := &Catalog{sites: []Website{{ID: "site", URL: "https://example.com", ThroughHub: true, Assignments: []Assignment{{Kind: "user", Value: "alice"}, {Kind: "user", Value: "bob"}}}},
		renderer: &renderer{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Empty(t, req.Header.Get("Authorization"))
			require.Empty(t, req.Header.Get("Cookie"))
			var input map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&input))
			keys = append(keys, input["session"].(string))
			require.Equal(t, "https://example.com", input["url"])
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"image":"frame"}`))}, nil
		})}}}
	router := echo.New()
	router.Use(gateway.WithContext())
	catalog.Register(router)
	check := func(user, key, body string, status int) {
		req := httptest.NewRequest("POST", Path+"/site/browser", strings.NewReader(body))
		(&gateway.Identity{ID: user, APIKeyID: key}).WriteTo(req.Header)
		req.Header.Set("Authorization", "Bearer hub-secret")
		req.Header.Set("Cookie", "hub-secret=1")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, status, recorder.Code, recorder.Body.String())
	}
	body := `{"session":"same-browser-tab-1234","action":"start"}`
	check("", "", body, 403)
	check("alice", "api-key", body, 403)
	check("mallory", "", body, 404)
	check("alice", "", `{"session":"short","action":"start"}`, 400)
	check("alice", "", `{"session":"same-browser-tab-1234","action":"evaluate"}`, 400)
	require.Empty(t, keys)
	check("alice", "", body, 200)
	check("bob", "", body, 200)
	require.NotEqual(t, keys[0], keys[1])
	check("alice", "", `{"session":"same-browser-tab-1234","action":"click","x":10,"y":20}`, 200)
	require.Equal(t, keys[0], keys[2])
	catalog.sites[0].Assignments = nil
	check("alice", "", body, 404)
	require.Len(t, keys, 3)
}

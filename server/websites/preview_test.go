package websites

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPreviewAuthorizationAndIsolation(t *testing.T) {
	fetches := 0
	catalog := &Catalog{
		sites: []Website{
			{ID: "assigned", URL: "http://internal.example:8080/report", ThroughHub: true, Assignments: []Assignment{{Kind: "user", Value: "member"}}},
			{ID: "script-only", URL: "http://internal.example:8080/app", ThroughHub: true, Assignments: []Assignment{{Kind: "user", Value: "member"}}},
			{ID: "attachment", URL: "http://internal.example:8080/download", ThroughHub: true, Assignments: []Assignment{{Kind: "user", Value: "member"}}},
			{ID: "direct", URL: "http://other.example/report", ThroughHub: false, Assignments: []Assignment{{Kind: "user", Value: "member"}}},
		},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			fetches++
			require.Empty(t, req.Header.Get("Authorization"))
			require.Empty(t, req.Header.Get("Cookie"))
			require.Contains(t, req.UserAgent(), "CloudMateHub-Preview")
			header := http.Header{"Content-Type": {"text/html"}}
			if req.URL.Path == "/app" {
				return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(`<div id="root"></div><script src="/assets/app.js"></script>`))}, nil
			}
			if req.URL.Path == "/download" {
				header.Set("Content-Disposition", "attachment; filename=report.html")
			} else {
				require.Equal(t, "http://internal.example:8080/report", req.URL.String())
			}
			return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(
				`<h1 onclick="bad()">Report</h1><script>bad()</script><img src="http://other.example"><form action="/write"><input></form>`,
			))}, nil
		})},
	}
	router := echo.New()
	router.Use(gateway.WithContext())
	catalog.Register(router)

	check := func(user, key, site string, expected int) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, Path+"/"+site+"/preview", nil)
		(&gateway.Identity{ID: user, APIKeyID: key}).WriteTo(req.Header)
		req.Header.Set("Authorization", "Bearer hub-only")
		req.Header.Set("Cookie", "hub-only=1")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, expected, recorder.Code)
		return recorder.Body.String()
	}

	check("", "", "assigned", 403)
	check("member", "key", "assigned", 403)
	check("other", "", "assigned", 404)
	check("member", "", "direct", 404)
	require.Zero(t, fetches)
	body := check("member", "", "assigned", 200)
	require.Equal(t, 1, fetches)
	require.Contains(t, body, "Report")
	require.Contains(t, check("member", "", "script-only", 422), "no readable content")
	require.Contains(t, check("member", "", "attachment", 415), "Downloads are disabled")
	for _, unsafe := range []string{"onclick", "bad()", "other.example", "action", "input"} {
		require.NotContains(t, body, unsafe)
	}
}

func TestPreviewBlocksPrivateNetwork(t *testing.T) {
	for _, target := range []string{"http://127.0.0.1:8080/", "http://10.1.2.3:8443/", "http://169.254.169.254/"} {
		response, err := previewClient().Get(target)
		require.Nil(t, response)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "connection refused")
	}
}

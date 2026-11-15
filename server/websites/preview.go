package websites

import (
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"code.dny.dev/ssrf"
	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
	nethtml "golang.org/x/net/html"
)

const previewLimit = 2 << 20

func previewClient() *http.Client {
	guard := ssrf.New(ssrf.WithAnyPort())
	dialer := net.Dialer{Timeout: 4 * time.Second, Control: guard.Safe}
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.User != nil {
				return http.ErrUseLastResponse
			}
			return nil
		},
		Transport: &http.Transport{
			MaxResponseHeaderBytes: 64 << 10,
			DialContext:            dialer.DialContext,
		},
	}
}

// Keep static content and discard attributes and executable or embedded content.
func staticHTML(body string) string {
	var out strings.Builder
	tokens := nethtml.NewTokenizer(strings.NewReader(body))
	skip := ""
	for {
		kind := tokens.Next()
		if kind == nethtml.ErrorToken {
			return out.String()
		}
		token := tokens.Token()
		if skip != "" {
			if kind == nethtml.EndTagToken && token.Data == skip {
				skip = ""
			}
			continue
		}
		if kind == nethtml.TextToken {
			out.WriteString(html.EscapeString(token.Data))
			continue
		}
		if kind != nethtml.StartTagToken && kind != nethtml.EndTagToken && kind != nethtml.SelfClosingTagToken {
			continue
		}
		switch token.Data {
		case "head", "script", "style", "iframe", "svg", "math", "template":
			if kind == nethtml.StartTagToken {
				skip = token.Data
			}
		case "h1", "h2", "h3", "h4", "p", "div", "span", "ul", "ol", "li", "table", "thead", "tbody",
			"tr", "th", "td", "br", "hr", "pre", "code", "strong", "em", "blockquote":
			out.WriteByte('<')
			if kind == nethtml.EndTagToken {
				out.WriteByte('/')
			}
			out.WriteString(token.Data)
			out.WriteByte('>')
		}
	}
}

func (c *Catalog) preview(g *gateway.Context) error {
	id, ok := g.GetID()
	if !ok || g.APIKeyID() != "" {
		return g.NoContent(http.StatusForbidden)
	}
	g.Response().Header().Set("Cache-Control", "no-store")
	for _, site := range c.list(g, id, g.Param("id")) {
		if !site.ThroughHub {
			continue
		}
		req, err := http.NewRequestWithContext(g.Ctx(), http.MethodGet, site.URL, nil)
		if err != nil {
			return g.NoContent(http.StatusBadGateway)
		}
		req.Header.Set("Accept", "text/html, text/plain")
		response, err := c.httpClient.Do(req)
		if err != nil {
			return g.NoContent(http.StatusBadGateway)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return g.JSON(http.StatusBadGateway, map[string]string{"error": "The website did not return a viewable page."})
		}
		disposition, _, _ := mime.ParseMediaType(response.Header.Get("Content-Disposition"))
		if strings.EqualFold(disposition, "attachment") {
			return g.JSON(http.StatusUnsupportedMediaType, map[string]string{"error": "Downloads are disabled for proxied websites."})
		}
		mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if mediaType != "text/html" && mediaType != "text/plain" {
			return g.JSON(http.StatusUnsupportedMediaType, map[string]string{"error": "Hub previews support HTML and plain text pages."})
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, previewLimit+1))
		if err != nil || len(body) > previewLimit {
			return g.JSON(http.StatusBadGateway, map[string]string{"error": "The website response could not be read or exceeded 2 MiB."})
		}
		content := "<pre>" + html.EscapeString(string(body)) + "</pre>"
		if mediaType == "text/html" {
			content = staticHTML(string(body))
		}
		return g.JSON(http.StatusOK, map[string]string{"name": site.Name, "html": content})
	}
	return g.NoContent(http.StatusNotFound)
}

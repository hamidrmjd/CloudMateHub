package websites

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"github.com/shellhub-io/shellhub/server/api/pkg/gateway"
)

var browserSessionID = regexp.MustCompile(`^[a-zA-Z0-9-]{16,64}$`)

// The renderer's session key is derived from authenticated identity and assignment,
// never from a client-provided owner. Every event rechecks the current assignment.
func (c *Catalog) browser(g *gateway.Context) error {
	id, ok := g.GetID()
	if !ok || g.APIKeyID() != "" {
		return g.NoContent(http.StatusForbidden)
	}
	g.Response().Header().Set("Cache-Control", "no-store")
	for _, site := range c.list(g, id, g.Param("id")) {
		if !site.ThroughHub {
			continue
		}
		if c.renderer == nil {
			return g.JSON(503, map[string]string{"error": "Interactive website browsing is not enabled on this Hub."})
		}
		var input struct {
			Session string  `json:"session"`
			Action  string  `json:"action"`
			X       float64 `json:"x"`
			Y       float64 `json:"y"`
			Text    string  `json:"text"`
		}
		if err := json.NewDecoder(io.LimitReader(g.Request().Body, 8193)).Decode(&input); err != nil || !browserSessionID.MatchString(input.Session) || len(input.Text) > 4096 {
			return g.NoContent(400)
		}
		switch input.Action {
		case "start", "frame", "click", "move", "wheel", "key", "text", "back", "forward", "reload", "close":
		default:
			return g.NoContent(400)
		}
		digest := sha256.Sum256([]byte(id + "\x00" + site.ID + "\x00" + input.Session))
		payload, _ := json.Marshal(map[string]any{"session": hex.EncodeToString(digest[:]), "action": input.Action, "url": site.URL, "x": input.X, "y": input.Y, "text": input.Text})
		req, err := http.NewRequestWithContext(g.Ctx(), http.MethodPost, "http://renderer/session", bytes.NewReader(payload))
		if err != nil {
			return g.NoContent(502)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := c.renderer.client.Do(req)
		if err != nil {
			return g.JSON(502, map[string]string{"error": "The remote browser is unavailable. Reopen the website to reconnect."})
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, previewLimit+1))
		if err != nil || len(body) > previewLimit {
			return g.NoContent(502)
		}
		return g.Blob(response.StatusCode, "application/json", body)
	}
	return g.NoContent(404)
}

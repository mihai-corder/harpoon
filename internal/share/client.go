// Package share uploads harpoon reports to a harpoon-share server (see
// server/README.md) and returns their public URLs.
package share

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultServer is the maintainer-hosted instance. Self-hosters override it
// with --server or HARPOON_SHARE_SERVER.
const DefaultServer = "https://harpoon-share.1962554030-517.workers.dev"

// Client talks to a harpoon-share server.
type Client struct {
	Server string
	HTTP   *http.Client
}

// Result is the server's response to an upload.
type Result struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	DeleteToken string `json:"delete_token"`
	ExpiresAt   string `json:"expires_at"`
}

// Upload posts the report HTML and returns the share result.
func (c *Client) Upload(html []byte, collection string) (Result, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}

	payload, err := json.Marshal(map[string]string{
		"html":       string(html),
		"collection": collection,
	})
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequest(http.MethodPost, c.Server+"/api/reports", bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "harpoon")

	res, err := c.HTTP.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf(
			"share server unreachable at %s: %w (note: *.workers.dev is unreachable from some networks — run your own instance from server/ and pass --server)",
			c.Server, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Result{}, err
	}

	if res.StatusCode != http.StatusCreated {
		return Result{}, fmt.Errorf("share server returned %d: %s", res.StatusCode, errorMessage(body))
	}

	var out Result
	if err := json.Unmarshal(body, &out); err != nil {
		return Result{}, fmt.Errorf("share server returned malformed JSON: %w", err)
	}
	if out.URL == "" || out.ID == "" {
		return Result{}, fmt.Errorf("share server response missing id/url")
	}
	return out, nil
}

// errorMessage extracts the server's {"error": "..."} body, falling back to
// the raw body.
func errorMessage(body []byte) string {
	var structured struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &structured); err == nil && structured.Error != "" {
		return structured.Error
	}
	trimmed := string(bytes.TrimSpace(body))
	if len(trimmed) > 200 {
		return trimmed[:200] + "..."
	}
	return trimmed
}

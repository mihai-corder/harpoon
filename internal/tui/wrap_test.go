package tui

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/mihai-corder/harpoon/internal/runner"
)

// TestResponseContentWrapsToViewport pins down the fix for long header values
// overflowing the pane: nothing rendered by the viewport may be wider than it.
func TestResponseContentWrapsToViewport(t *testing.T) {
	m := readyModel()
	m.respVP.Width = 40

	res := runner.Result{
		Name: "wide", Method: "GET", URL: "https://api.example.com/wide",
		Status: 200, Duration: 12 * time.Millisecond,
		Headers: http.Header{
			"Access-Control-Expose-Headers": []string{
				"ETag, Link, Location, Retry-After, X-GitHub-OTP, X-RateLimit-Limit, X-RateLimit-Remaining"},
		},
		Body: []byte("ok"),
	}

	m.syncViewportContentWith(res)

	for i, line := range strings.Split(m.respVP.View(), "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 40,
			"viewport line %d overflows the pane: %q", i, line)
	}
}

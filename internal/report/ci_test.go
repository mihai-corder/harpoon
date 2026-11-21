package report

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihai-corder/harpoon/internal/runner"
)

func fakeCIResults() []runner.Result {
	return []runner.Result{
		{
			Name: "login", Method: "POST", URL: "http://api.example.com/login",
			Status: 200, Duration: 42 * time.Millisecond,
			Extracted: map[string]string{"token": "abc"},
			Checks: []runner.Check{
				{Kind: "status", OK: true, Detail: "status is 200"},
			},
		},
		{
			Name: "list-users", Method: "GET", URL: "http://api.example.com/users",
			Status: 500, Duration: 38 * time.Millisecond,
			Failures: []string{"expected status 200, got 500"},
			Checks: []runner.Check{
				{Kind: "status", OK: false, Detail: "expected status 200, got 500"},
			},
		},
	}
}

func TestBuildCIReportCountsAndFields(t *testing.T) {
	started := time.Now().Add(-1500 * time.Millisecond)
	rep := BuildCIReport("demo", started, fakeCIResults())

	assert.Equal(t, "demo", rep.Collection)
	assert.Equal(t, 1, rep.Passed)
	assert.Equal(t, 1, rep.Failed)
	assert.Equal(t, 2, rep.Total())
	require.Len(t, rep.Results, 2)

	login := rep.Results[0]
	assert.Equal(t, "login", login.Name)
	assert.Equal(t, 200, login.Status)
	require.Len(t, login.Checks, 1)
	assert.True(t, login.Checks[0].OK)
	assert.GreaterOrEqual(t, rep.DurationMS, 1000.0, "wall time should include the full run")
}

func TestCIReportJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	rep := BuildCIReport("demo", time.Now(), fakeCIResults())
	require.NoError(t, rep.WriteJSON(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var parsed CIReport
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, rep.Passed, parsed.Passed)
	assert.Equal(t, rep.Failed, parsed.Failed)
	require.Len(t, parsed.Results, 2)
	assert.Equal(t, 500, parsed.Results[1].Status)
	assert.Contains(t, parsed.Results[1].Failures, "expected status 200, got 500")

	raw := string(data)
	assert.Contains(t, raw, `"collection": "demo"`)
	assert.Contains(t, raw, `"duration_ms"`)
}

func TestCIReportHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	rep := BuildCIReport("demo", time.Now(), fakeCIResults())
	require.NoError(t, rep.WriteHTML(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	html := string(data)

	assert.Contains(t, html, "harpoon · demo")
	assert.Contains(t, html, "1 passed")
	assert.Contains(t, html, "1 failed")
	assert.Contains(t, html, "b-ok")
	assert.Contains(t, html, "b-fail")
	assert.Contains(t, html, "expected status 200, got 500")
	assert.Contains(t, html, "<!doctype html>")

	// HTML escaping: a failure message with markup must not break the page.
	evil := []runner.Result{{
		Name:     `<script>alert(1)</script>`,
		Method:   "GET",
		URL:      "http://x",
		Status:   0,
		Err:      errors.New(`<img src=x onerror=alert(2)>`),
		Failures: []string{`<b>bold</b>`},
		Checks: []runner.Check{
			{Kind: "headers", OK: true, Detail: "has " + http.CanonicalHeaderKey("x-test")},
		},
	}}
	out := BuildCIReport("demo", time.Now(), evil).HTML()
	assert.NotContains(t, out, "<script>alert(1)</script>")
	assert.Contains(t, out, "&lt;script&gt;")
	assert.NotContains(t, out, "<img src=x")
}

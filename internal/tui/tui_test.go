package tui

import (
	"net/http"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/runner"
)

func fakeColl() *config.Collection {
	return &config.Collection{
		Name: "github-demo",
		Requests: []config.NamedRequest{
			{Name: "zen", Request: config.Request{Method: "GET", URL: "https://api.github.com/zen"}},
			{Name: "user-info", Request: config.Request{Method: "GET", URL: "https://api.github.com/users/{{user}}"}},
			{Name: "first-repo", Request: config.Request{Method: "GET", URL: "https://api.github.com/users/{{login}}/repos"}},
		},
	}
}

func fakeResults() []runner.Result {
	return []runner.Result{
		{
			Name: "zen", Method: "GET", URL: "https://api.github.com/zen",
			Status: 200, Duration: 218 * time.Millisecond,
			Headers: http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:    []byte("Keep the curve flat."),
			Checks: []runner.Check{
				{Kind: "status", OK: true, Detail: "status is 200"},
				{Kind: "max_ms", OK: true, Detail: "took 218ms (max 3000ms)"},
			},
		},
		{
			Name: "user-info", Method: "GET", URL: "https://api.github.com/users/octocat",
			Status: 200, Duration: 156 * time.Millisecond,
			Headers:   http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Body:      []byte(`{"login":"octocat","id":583231}`),
			Extracted: map[string]string{"login": "octocat"},
			Checks:    []runner.Check{{Kind: "status", OK: true, Detail: "status is 200"}},
		},
		{
			Name: "first-repo", Method: "GET", URL: "https://api.github.com/users/octocat/repos",
			Status: 500, Duration: 89 * time.Millisecond,
			Headers: http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Body:    []byte(`{"message": "Server Error"}`),
			Checks: []runner.Check{
				{Kind: "status", OK: false, Detail: "expected status 200, got 500"},
				{Kind: "contains", OK: true, Detail: `body contains "octocat"`},
			},
			Failures: []string{"expected status 200, got 500"},
		},
	}
}

// key builds a KeyMsg the same way terminal input would arrive.
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func send(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func readyModel() Model {
	m := New(fakeColl(), nil, nil)
	m.width, m.height, m.ready = 100, 30, true
	m.syncSizes()
	return m
}

func TestCursorMovementClampsAtBounds(t *testing.T) {
	m := readyModel()
	m = send(m, key("j"), key("j"))
	assert.Equal(t, 2, m.cursor)
	m = send(m, key("k"))
	assert.Equal(t, 1, m.cursor)
	m = send(m, key("j"), key("j"), key("j"), key("j"))
	assert.Equal(t, 2, m.cursor, "cursor must clamp at the last request")
	m = send(m, key("k"), key("k"), key("k"), key("k"))
	assert.Equal(t, 0, m.cursor, "cursor must clamp at the first request")
}

func TestTabTogglesFocus(t *testing.T) {
	m := readyModel()
	assert.Equal(t, focusRequests, m.focus)
	m = send(m, key("tab"))
	assert.Equal(t, focusResponse, m.focus)
	m = send(m, key("tab"))
	assert.Equal(t, focusRequests, m.focus)
}

func TestEnterStartsBackgroundRun(t *testing.T) {
	m := readyModel()
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	require.NotNil(t, cmd, "enter should schedule a background run")
	assert.Equal(t, "zen", m.running)
}

func TestRunAllKey(t *testing.T) {
	m := readyModel()
	next, cmd := m.Update(key("R"))
	m = next.(Model)
	require.NotNil(t, cmd)
	assert.Equal(t, "all", m.running)
}

func TestRunDoneStoresResultsAndHistory(t *testing.T) {
	m := readyModel()
	m = send(m, runDoneMsg{results: fakeResults()})

	assert.Empty(t, m.running)
	assert.Len(t, m.history, 3)

	zen, ok := m.results["zen"]
	require.True(t, ok)
	assert.Equal(t, 200, zen.Status)

	assert.Contains(t, m.notice, "2/3")
}

func TestHistoryToggleAndClose(t *testing.T) {
	m := readyModel()
	m = send(m, runDoneMsg{results: fakeResults()[:1]}, key("h"))
	assert.True(t, m.showHist)
	m = send(m, key("esc"))
	assert.False(t, m.showHist)
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := readyModel().Update(key(k))
		require.NotNil(t, cmd, "%s should quit", k)
	}
}

func TestSecondRunIgnoredWhileBusy(t *testing.T) {
	m := readyModel()
	m = send(m, key("R"))
	m = send(m, key("enter"))
	assert.Equal(t, "all", m.running, "a run must not be replaced while in flight")
	assert.Contains(t, m.notice, "already in progress")
}

func TestResponseViewSections(t *testing.T) {
	out := responseView(fakeResults()[1])
	assert.Contains(t, out, "ASSERTIONS")
	assert.Contains(t, out, "✓ status is 200")
	assert.Contains(t, out, "EXTRACTS")
	assert.Contains(t, out, "login = octocat")
	assert.Contains(t, out, "HEADERS")
	assert.Contains(t, out, "BODY")
	assert.Contains(t, out, `"login"`, "JSON body should be pretty-printed")

	fail := responseView(fakeResults()[2])
	assert.Contains(t, fail, "✗ expected status 200, got 500")
	assert.Contains(t, fail, "500 Internal Server Error")
}

func TestResponseViewTransportError(t *testing.T) {
	res := runner.Result{Name: "x", Method: "GET", URL: "http://127.0.0.1:1/x",
		Err: assert.AnError}
	out := responseView(res)
	assert.Contains(t, out, "✗")
	assert.NotContains(t, out, "ASSERTIONS")
}

// TestSnapshotViewsForManualInspection renders the TUI's key states so the
// frames can be eyeballed without a terminal (and compared across changes).
// Run with: go test -run TestSnapshotViews -v ./internal/tui/
func TestSnapshotViewsForManualInspection(t *testing.T) {
	m := readyModel()
	t.Log("FRAME 1 — initial state:\n" + m.View())

	m = send(m, runDoneMsg{results: fakeResults()})
	t.Log("FRAME 2 — after R (run all), first request selected:\n" + m.View())

	m = send(m, key("down"))
	t.Log("FRAME 3 — cursor on user-info:\n" + m.View())

	m = send(m, key("h"))
	t.Log("FRAME 4 — history overlay:\n" + m.View())
}

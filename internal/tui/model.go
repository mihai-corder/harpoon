// Package tui provides harpoon's interactive terminal interface: pick a
// request, run it, inspect the response — using the exact same collection
// files the CLI and CI consume.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/runner"
)

type focus int

const (
	focusRequests focus = iota
	focusResponse
)

// runDoneMsg delivers the results of a background run to the UI.
type runDoneMsg struct{ results []runner.Result }

// historyEntry is one request execution recorded during this session.
type historyEntry struct {
	at  time.Time
	res runner.Result
}

type Model struct {
	coll    *config.Collection
	sess    *runner.Session
	profile map[string]string
	cliVars map[string]string

	cursor  int
	focus   focus
	results map[string]runner.Result // latest result per request name

	respVP  viewport.Model
	histVP  viewport.Model
	history []historyEntry

	running  string // request name or "all" while a run is in flight
	spinner  spinner.Model
	showHist bool
	notice   string // transient status line (e.g. "2/3 passed")

	width  int
	height int
	ready  bool
}

// New builds the initial model for a collection.
func New(coll *config.Collection, profile, cliVars map[string]string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return Model{
		coll:    coll,
		sess:    runner.NewSession(),
		profile: profile,
		cliVars: cliVars,
		results: make(map[string]runner.Result),
		respVP:  viewport.New(80, 20),
		histVP:  viewport.New(60, 10),
		spinner: sp,
	}
}

func (m Model) Init() tea.Cmd {
	// Own the window/tab title instead of inheriting the shell's command line.
	return tea.SetWindowTitle("harpoon · " + m.coll.Name)
}

// current returns the request under the cursor.
func (m Model) current() config.NamedRequest {
	return m.coll.Requests[m.cursor]
}

// currentResult returns the latest result for the selected request, if any.
func (m Model) currentResult() (runner.Result, bool) {
	r, ok := m.results[m.current().Name]
	return r, ok
}

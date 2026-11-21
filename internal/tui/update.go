package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/runner"
)

// Update advances the model by one message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.syncSizes()
		m.syncViewportContent()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.running != "" {
			return m, cmd // keep the spinner alive only while a run is in flight
		}
		return m, nil

	case runDoneMsg:
		m.running = ""
		m.applyResults(msg.results)
		m.syncViewportContent()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) applyResults(results []runner.Result) {
	passed := 0
	for _, r := range results {
		m.results[r.Name] = r
		m.history = append(m.history, historyEntry{at: timeNow(), res: r})
		if r.Passed() {
			passed++
		}
	}

	switch {
	case len(results) == 1 && results[0].Passed():
		m.notice = ""
	case len(results) == 1:
		m.notice = results[0].Name + " failed — see response pane"
	default:
		m.notice = fmt.Sprintf("%d/%d passed", passed, len(results))
	}
	m.syncHistoryContent()
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "q" || key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.showHist {
		switch key {
		case "esc", "h":
			m.showHist = false
		case "up", "k":
			m.histVP.LineUp(1)
		case "down", "j":
			m.histVP.LineDown(1)
		}
		return m, nil
	}

	switch key {
	case "tab":
		if m.focus == focusRequests {
			m.focus = focusResponse
		} else {
			m.focus = focusRequests
		}

	case "up", "k":
		if m.focus == focusRequests {
			if m.cursor > 0 {
				m.cursor--
				m.syncViewportContent()
			}
		} else {
			m.respVP.LineUp(1)
		}

	case "down", "j":
		if m.focus == focusRequests {
			if m.cursor < len(m.coll.Requests)-1 {
				m.cursor++
				m.syncViewportContent()
			}
		} else {
			m.respVP.LineDown(1)
		}

	case "ctrl+u":
		m.respVP.HalfPageUp()
	case "ctrl+d":
		m.respVP.HalfPageDown()

	case "h":
		m.showHist = true
		m.syncHistoryContent()

	case "enter":
		if m.running != "" {
			m.notice = "a run is already in progress"
			return m, nil
		}
		m.running = m.current().Name
		one := &config.Collection{Name: m.coll.Name, Vars: m.coll.Vars,
			Requests: []config.NamedRequest{m.current()}}
		return m, m.startRun(one)

	case "R":
		if m.running != "" {
			m.notice = "a run is already in progress"
			return m, nil
		}
		m.running = "all"
		return m, m.startRun(m.coll)
	}
	return m, nil
}

// startRun launches a run in the background, spinner alongside it. The
// session carries extracted variables, so chained requests work across
// individual runs.
func (m Model) startRun(coll *config.Collection) tea.Cmd {
	sess, profile, cliVars := m.sess, m.profile, m.cliVars
	tick := m.spinner.Tick
	run := func() tea.Msg {
		return runDoneMsg{results: sess.Run(coll, profile, runner.Options{CLIVars: cliVars})}
	}
	return tea.Batch(tick, run)
}

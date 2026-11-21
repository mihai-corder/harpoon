package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// timeNow is indirected so tests can pin timestamps.
var timeNow = time.Now

const (
	headerLines = 1
	footerLines = 1
	leftMinW    = 24
	leftMaxW    = 38
)

var (
	appTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("57"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	okStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	selStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("236"))
	paneTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	boxStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
	boxFocusStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
	histTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("130"))
)

var methodColors = map[string]lipgloss.Color{
	"GET":     "10",
	"POST":    "11",
	"PUT":     "12",
	"PATCH":   "12",
	"DELETE":  "9",
	"HEAD":    "13",
	"OPTIONS": "13",
}

func methodStyle(method string) lipgloss.Style {
	color, ok := methodColors[method]
	if !ok {
		color = "15"
	}
	return lipgloss.NewStyle().Bold(true).Foreground(color)
}

func (m Model) View() string {
	if !m.ready {
		return "harpoon — waiting for window size…"
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.renderRequestsPane(), m.renderResponsePane())
	page := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(), body, m.renderFooter())

	if m.showHist {
		page = m.overlayHistory(page)
	}
	return page
}

func (m Model) renderHeader() string {
	left := fmt.Sprintf("harpoon · %s · %d requests", m.coll.Name, len(m.coll.Requests))
	var right string
	switch {
	case m.running != "":
		right = m.spinner.View() + " running " + m.running
	case m.notice != "":
		right = m.notice
	}

	inner := m.width - 2 // header padding
	// 2 cells of slack: ambiguous-width runes (·) can measure differently
	// under East Asian width rules and would otherwise wrap the right side.
	pad := inner - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if pad < 1 {
		pad = 1
	}
	return appTitleStyle.Padding(0, 1).Width(inner).Render(left + strings.Repeat(" ", pad) + right)
}

func (m Model) renderFooter() string {
	return dimStyle.Padding(0, 1).Render(
		"↑/k ↓/j move · tab focus · enter run · R run all · h history · q quit")
}

// layout computes the box outer widths and the shared body height.
func (m Model) layout() (leftBoxW, rightBoxW, bodyH int) {
	leftBoxW = 3 * m.width / 10
	if leftBoxW < leftMinW+2 {
		leftBoxW = leftMinW + 2
	}
	if leftBoxW > leftMaxW+2 {
		leftBoxW = leftMaxW + 2
	}
	if leftBoxW > m.width-10 {
		leftBoxW = m.width - 10
	}
	rightBoxW = m.width - leftBoxW
	bodyH = m.height - headerLines - footerLines
	return
}

func (m Model) renderRequestsPane() string {
	leftBoxW, _, bodyH := m.layout()
	innerW, innerH := leftBoxW-2, bodyH-2

	var b strings.Builder
	b.WriteString(paneTitleStyle.Render("REQUESTS"))
	b.WriteString("\n")

	for i, nr := range m.coll.Requests {
		marker := "  "
		if i == m.cursor {
			marker = "▸ "
		}
		dot := dimStyle.Render("·")
		if r, ok := m.results[nr.Name]; ok {
			if r.Passed() {
				dot = okStyle.Render("✓")
			} else {
				dot = errStyle.Render("✗")
			}
		}
		line := fmt.Sprintf("%s%s %s %s", marker, methodStyle(nr.Request.Method).Render(nr.Request.Method), dot, nr.Name)
		line = truncate(line, innerW)
		if i == m.cursor {
			line = selStyle.Width(innerW).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	content := padLines(b.String(), innerH)
	style := boxStyle
	if m.focus == focusRequests {
		style = boxFocusStyle
	}
	return style.Width(leftBoxW - 2).Height(innerH).Render(content)
}

func (m Model) renderResponsePane() string {
	_, rightBoxW, bodyH := m.layout()
	innerW, innerH := rightBoxW-2, bodyH-2

	var b strings.Builder
	if m.showHintInResponse() {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"No response yet — press enter to run %q, R to run all.",
			m.current().Name)))
	} else {
		b.WriteString(m.respVP.View())
	}

	style := boxStyle
	if m.focus == focusResponse {
		style = boxFocusStyle
	}
	return style.Width(innerW).Height(innerH).Render(b.String())
}

func (m Model) showHintInResponse() bool {
	_, ok := m.currentResult()
	return !ok
}

func (m Model) overlayHistory(base string) string {
	hw := min(m.width-8, 72)
	hh := min(m.height-8, 14)

	box := boxStyle.Width(hw - 2).Height(hh - 2).Render(m.histVP.View())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// syncSizes re-applies the current window size to both viewports.
func (m *Model) syncSizes() {
	_, rightBoxW, bodyH := m.layout()
	m.respVP.Width = rightBoxW - 2
	m.respVP.Height = bodyH - 2
}

// syncViewportContent rebuilds the response pane for the selected request.
func (m *Model) syncViewportContent() {
	if r, ok := m.currentResult(); ok {
		m.respVP.SetContent(responseView(r))
		m.respVP.GotoTop()
	}
}

// syncHistoryContent rebuilds the history overlay's scrollable content,
// newest runs first.
func (m *Model) syncHistoryContent() {
	hw := min(m.width-8, 72)

	var b strings.Builder
	b.WriteString(histTitleStyle.Render(" HISTORY (this session) ") + "\n")
	if len(m.history) == 0 {
		b.WriteString(dimStyle.Render("no runs yet") + "\n")
	}
	for i := len(m.history) - 1; i >= 0; i-- {
		e := m.history[i]
		mark, style := "✓", okStyle
		if !e.res.Passed() {
			mark, style = "✗", errStyle
		}
		status := fmt.Sprintf("%d", e.res.Status)
		if e.res.Status == 0 {
			status = "-"
		}
		line := fmt.Sprintf("%s  %s  %s  %s  %-6s %s",
			e.at.Format("15:04:05"), style.Render(mark),
			methodStyle(e.res.Method).Render(e.res.Method), e.res.Name,
			status, e.res.Duration.Truncate(time.Millisecond))
		b.WriteString(truncate(line, hw-2))
		b.WriteString("\n")
	}

	m.histVP.SetContent(b.String())
	m.histVP.GotoTop()
}

// padLines pads a block with blank lines to exactly n lines.
func padLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// truncate cuts a string to at most w display cells, keeping ANSI styling.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}

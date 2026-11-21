package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/mihai-corder/harpoon/internal/runner"
)

// maxDisplayBody caps how much of the body is rendered in the pane.
const maxDisplayBody = 16 << 10

// sectionTitle renders a small section heading.
func sectionTitle(s string) string {
	return paneTitleStyle.Render(s)
}

// statusStyle colors the status line by response class.
func statusStyle(status int) lipgloss.Style {
	switch {
	case status >= 200 && status < 300:
		return okStyle
	case status >= 400:
		return errStyle
	default:
		return warnStyle
	}
}

// responseView renders the full response pane content for one result.
func responseView(r runner.Result) string {
	var b strings.Builder

	b.WriteString(methodStyle(r.Method).Render(r.Method) + " " + r.URL + "\n\n")

	if r.Err != nil {
		b.WriteString(errStyle.Render("✗ "+r.Err.Error()) + "\n")
		return b.String()
	}

	b.WriteString(statusStyle(r.Status).Render(fmt.Sprintf("%d %s", r.Status, http.StatusText(r.Status))) +
		dimStyle.Render(fmt.Sprintf(" · %s · %s",
			r.Duration.Truncate(time.Millisecond), humanBytes(len(r.Body)))) + "\n")

	if len(r.Checks) > 0 {
		b.WriteString("\n" + sectionTitle("ASSERTIONS") + "\n")
		for _, c := range r.Checks {
			mark, style := "✓", okStyle
			if !c.OK {
				mark, style = "✗", errStyle
			}
			b.WriteString(style.Render(mark+" "+c.Detail) + "\n")
		}
	}

	if len(r.Extracted) > 0 {
		b.WriteString("\n" + sectionTitle("EXTRACTS") + "\n")
		names := sortedKeys(r.Extracted)
		for _, k := range names {
			b.WriteString(okStyle.Render("+ ") + k + dimStyle.Render(" = ") + r.Extracted[k] + "\n")
		}
	}

	if len(r.Headers) > 0 {
		b.WriteString("\n" + sectionTitle("HEADERS") + "\n")
		for _, k := range sortedHeaderKeys(r.Headers) {
			b.WriteString(dimStyle.Render(strings.ToLower(k)+": ") + strings.Join(r.Headers.Values(k), ", ") + "\n")
		}
	}

	b.WriteString("\n" + sectionTitle("BODY") + "\n")
	b.WriteString(renderBody(r.Body))
	return b.String()
}

// renderBody pretty-prints JSON bodies; everything else is shown verbatim.
func renderBody(body []byte) string {
	if len(body) == 0 {
		return dimStyle.Render("(empty)")
	}

	shown := body
	var indented bytes.Buffer
	if err := json.Indent(&indented, body, "", "  "); err == nil {
		shown = indented.Bytes()
	}

	truncated := false
	if len(shown) > maxDisplayBody {
		shown = shown[:maxDisplayBody]
		truncated = true
	}

	out := string(shown)
	if truncated {
		out += "\n" + warnStyle.Render("… truncated (full body in CI reports from v0.3)")
	}
	return out
}

func humanBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedHeaderKeys(h http.Header) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Package report renders run results as a terminal summary table.
package report

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mihai-corder/harpoon/internal/runner"
)

const (
	ansiGreen  = "\x1b[32m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiReset  = "\x1b[0m"
)

// Print writes a summary table for the results to w. ANSI colors are used
// only when w is a terminal and the NO_COLOR environment variable is unset.
func Print(w io.Writer, results []runner.Result) {
	color := supportsColor(w)

	nameW, methodW, urlW, statusW, timeW := widths(results)

	var passed, failed int
	var total time.Duration
	for _, r := range results {
		total += r.Duration
		if r.Passed() {
			passed++
		} else {
			failed++
		}
	}

	header := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
		nameW, "NAME", methodW, "METHOD", urlW, "URL", statusW, "STATUS", timeW, "TIME", "RESULT")
	fmt.Fprint(w, header)

	for _, r := range results {
		status := fmt.Sprintf("%d", r.Status)
		if r.Status == 0 {
			status = "-"
		}
		url := r.URL
		if len(url) > urlW {
			url = url[:urlW-3] + "..."
		}
		resCell, resColor := "ok", ansiGreen
		if !r.Passed() {
			resCell, resColor = "failed", ansiRed
		}
		if color {
			resCell = resColor + resCell + ansiReset
		}

		fmt.Fprintf(w, "  %-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
			nameW, r.Name, methodW, r.Method, urlW, url, statusW, status, timeW, dur(r.Duration), resCell)

		for _, f := range r.Failures {
			msg := "  " + strings.Repeat(" ", nameW) + "  ✗ " + f
			if color {
				msg = ansiRed + msg + ansiReset
			}
			fmt.Fprintln(w, msg)
		}
		if r.Err != nil {
			msg := "  " + strings.Repeat(" ", nameW) + "  ✗ " + r.Err.Error()
			if color {
				msg = ansiRed + msg + ansiReset
			}
			fmt.Fprintln(w, msg)
		}
	}

	summary := fmt.Sprintf("\n%d passed, %d failed in %s\n", passed, failed, dur(total))
	if failed > 0 && color {
		summary = ansiRed + summary + ansiReset
	} else if failed == 0 && passed > 0 && color {
		summary = ansiGreen + summary + ansiReset
	}
	fmt.Fprint(w, summary)
}

// widths computes per-column paddings; the URL column is capped so long
// query strings do not push the table off screen.
func widths(results []runner.Result) (nameW, methodW, urlW, statusW, timeW int) {
	const maxURL = 60
	nameW, methodW, urlW, statusW, timeW = 4, 6, 3, 6, 4 // header lengths
	for _, r := range results {
		if l := len([]rune(r.Name)); l > nameW {
			nameW = l
		}
		if l := len(r.Method); l > methodW {
			methodW = l
		}
		if l := len(r.URL); l > urlW {
			urlW = l
		}
	}
	if urlW > maxURL {
		urlW = maxURL
	}
	return
}

// dur renders a duration at terminal-friendly precision.
func dur(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return d.Truncate(time.Microsecond).String()
	case d < time.Second:
		return d.Truncate(time.Millisecond).String()
	default:
		return d.Truncate(10 * time.Millisecond).String()
	}
}

func supportsColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

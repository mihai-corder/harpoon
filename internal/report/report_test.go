package report

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mihai-corder/harpoon/internal/runner"
)

func TestPrintSummaryTable(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, []runner.Result{
		{Name: "login", Method: "POST", URL: "http://api.example.com/auth/login", Status: 200, Duration: 42 * time.Millisecond},
		{Name: "list-users", Method: "GET", URL: "http://api.example.com/users", Status: 200, Duration: 38 * time.Millisecond},
	})

	out := buf.String()
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "login")
	assert.Contains(t, out, "POST")
	assert.Contains(t, out, "http://api.example.com/auth/login")
	assert.Contains(t, out, "200")
	assert.Contains(t, out, "42ms")
	assert.Contains(t, out, "2 passed, 0 failed in 80ms")
}

func TestPrintFailureDetails(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, []runner.Result{
		{
			Name: "login", Method: "POST", URL: "http://api.example.com/auth/login",
			Status: 500, Duration: 10 * time.Millisecond,
			Failures: []string{"expected status 200, got 500"},
		},
		{
			Name: "broken", Method: "GET", URL: "http://api.example.com/x",
			Status: 0, Duration: time.Millisecond,
			Err: errors.New("request failed: connection refused"),
		},
	})

	out := buf.String()
	assert.Contains(t, out, "✗ expected status 200, got 500")
	assert.Contains(t, out, "✗ request failed: connection refused")
	assert.Contains(t, out, "0 passed, 2 failed")
}

func TestPrintTruncatesLongURLs(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, []runner.Result{
		{Name: "long", Method: "GET", URL: "http://example.com/" + string(make([]byte, 200)), Status: 200},
	})
	out := buf.String()
	assert.Less(t, lenOfLongestLine(out), 200, "URL column should be capped")
}

func lenOfLongestLine(s string) int {
	longest := 0
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			if i-start > longest {
				longest = i - start
			}
			start = i + 1
		}
	}
	return longest
}

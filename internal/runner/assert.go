package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mihai-corder/harpoon/internal/config"
)

// Check is the outcome of a single assertion.
type Check struct {
	Kind   string // "status", "contains" or "max_ms"
	OK     bool
	Detail string
}

// evaluateExpect checks the expectations against the final response and
// returns one Check per specified assertion, in a stable order.
func evaluateExpect(e *config.Expect, status int, body []byte, took time.Duration) []Check {
	var checks []Check

	if e.Status != 0 {
		ok := status == e.Status
		detail := fmt.Sprintf("expected status %d, got %d", e.Status, status)
		if ok {
			detail = fmt.Sprintf("status is %d", status)
		}
		checks = append(checks, Check{Kind: "status", OK: ok, Detail: detail})
	}
	if e.Contains != "" {
		ok := strings.Contains(string(body), e.Contains)
		detail := fmt.Sprintf("body does not contain %q", e.Contains)
		if ok {
			detail = fmt.Sprintf("body contains %q", e.Contains)
		}
		checks = append(checks, Check{Kind: "contains", OK: ok, Detail: detail})
	}
	if e.MaxMS > 0 {
		ms := took.Milliseconds()
		ok := ms <= e.MaxMS
		detail := fmt.Sprintf("took %dms, max allowed %dms", ms, e.MaxMS)
		if ok {
			detail = fmt.Sprintf("took %dms (max %dms)", ms, e.MaxMS)
		}
		checks = append(checks, Check{Kind: "max_ms", OK: ok, Detail: detail})
	}
	return checks
}

// response resolves {{response.*}} references against one received response.
type response struct {
	status  int
	headers http.Header
	body    []byte

	parsed any
	perr   error
}

// resolve returns the value for a response reference such as
// response.status, response.headers.Name or response.body.a.b.0.
func (r *response) resolve(name string) (string, bool) {
	switch {
	case name == "response.status":
		return strconv.Itoa(r.status), true

	case strings.HasPrefix(name, "response.headers."):
		key := strings.TrimPrefix(name, "response.headers.")
		vals := r.headers.Values(key) // canonical, case-insensitive lookup
		if len(vals) == 0 {
			return "", false
		}
		return vals[0], true

	case strings.HasPrefix(name, "response.body."):
		path := strings.TrimPrefix(name, "response.body.")
		root, err := r.parse()
		if err != nil {
			return "", false
		}
		cur := root
		for _, seg := range strings.Split(path, ".") {
			switch t := cur.(type) {
			case map[string]any:
				v, ok := t[seg]
				if !ok {
					return "", false
				}
				cur = v
			case []any:
				idx, err := strconv.Atoi(seg)
				if err != nil || idx < 0 || idx >= len(t) {
					return "", false
				}
				cur = t[idx]
			default:
				return "", false
			}
		}
		return scalarString(cur)
	}
	return "", false
}

// parse lazily decodes the body as JSON, preserving number formatting.
func (r *response) parse() (any, error) {
	if r.parsed == nil && r.perr == nil {
		dec := json.NewDecoder(bytes.NewReader(r.body))
		dec.UseNumber()
		r.perr = dec.Decode(&r.parsed)
	}
	return r.parsed, r.perr
}

// scalarString converts a JSON leaf value to its string form.
func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case json.Number:
		return t.String(), true
	case bool:
		return strconv.FormatBool(t), true
	case nil:
		return "", true
	default:
		return "", false // objects and arrays are not extractable scalars
	}
}

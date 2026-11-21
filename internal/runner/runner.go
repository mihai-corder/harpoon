// Package runner executes a collection's requests in order: it expands
// variables, chains values extracted from responses, and evaluates
// assertions.
//
// Variable resolution order (highest wins):
//
//	CLI vars  >  extracted values  >  request-level vars  >  profile  >  collection vars
package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/interpolate"
)

// maxBodyBytes caps how much of a response body is read.
const maxBodyBytes = 10 << 20

// Options controls a run.
type Options struct {
	// CLIVars override every other variable source.
	CLIVars map[string]string
	// Client sends the HTTP requests; nil means a 30s-timeout default.
	Client *http.Client
}

// Result captures the outcome of one request.
type Result struct {
	Name     string
	Method   string
	URL      string
	Status   int
	Duration time.Duration
	// Err is a transport or expansion failure; the request never completed.
	Err error
	// Failures holds assertion violation messages. A request without Expect
	// passes as long as the HTTP exchange itself succeeds.
	Failures []string
}

// Passed reports whether the request completed and every assertion held.
func (r Result) Passed() bool { return r.Err == nil && len(r.Failures) == 0 }

// Run executes the collection's requests in order and returns one Result per
// request. Values extracted from responses are visible to later requests.
func Run(coll *config.Collection, profile map[string]string, opts Options) []Result {
	if profile == nil {
		profile = map[string]string{}
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Second}
	}

	// Base scope: collection vars, overridden by the environment profile.
	base := mergeMaps(coll.Vars, profile)
	carry := map[string]string{} // values extracted by earlier requests

	results := make([]Result, 0, len(coll.Requests))
	for _, nr := range coll.Requests {
		res, extracted := runRequest(nr, base, carry, opts)
		for k, v := range extracted {
			carry[k] = v
		}
		results = append(results, res)
	}
	return results
}

func runRequest(nr config.NamedRequest, base, carry map[string]string, opts Options) (Result, map[string]string) {
	res := Result{Name: nr.Name, Method: nr.Request.Method}

	// Outer scope (collection + profile + carried + CLI) is what request-level
	// vars may reference; CLI vars sit on top so they override everything.
	outer := mergeMaps(mergeMaps(base, carry), opts.CLIVars)

	reqVars, err := expandVars(nr.Request.Vars, outer)
	if err != nil {
		res.Err = err
		return res, nil
	}

	// Final scope for templates: CLI > carried > request vars > base.
	scope := mergeMaps(mergeMaps(mergeMaps(base, reqVars), carry), opts.CLIVars)
	resolve := func(name string) (string, bool) {
		v, ok := scope[name]
		return v, ok
	}

	url, err := interpolate.Interpolate(nr.Request.URL, resolve)
	if err != nil {
		res.Err = fmt.Errorf("expand url: %w", err)
		return res, nil
	}
	res.URL = url

	headers := make(map[string]string, len(nr.Request.Headers))
	for k, v := range nr.Request.Headers {
		expanded, err := interpolate.Interpolate(v, resolve)
		if err != nil {
			res.Err = fmt.Errorf("expand header %q: %w", k, err)
			return res, nil
		}
		headers[k] = expanded
	}

	var bodyReader io.Reader
	if nr.Request.Body != nil {
		rendered, err := interpolate.WalkStrings(nr.Request.Body, func(s string) (string, error) {
			return interpolate.Interpolate(s, resolve)
		})
		if err != nil {
			res.Err = fmt.Errorf("expand body: %w", err)
			return res, nil
		}
		payload, err := json.Marshal(rendered)
		if err != nil {
			res.Err = fmt.Errorf("encode body as JSON: %w", err)
			return res, nil
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(nr.Request.Method, url, bodyReader)
	if err != nil {
		res.Err = fmt.Errorf("build request: %w", err)
		return res, nil
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "harpoon/0.1")
	}
	if nr.Request.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := opts.Client.Do(req)
	res.Duration = time.Since(start)
	if err != nil {
		res.Err = fmt.Errorf("request failed: %w", err)
		return res, nil
	}
	defer resp.Body.Close()

	res.Status = resp.StatusCode
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		res.Err = fmt.Errorf("read response: %w", err)
		return res, nil
	}

	rx := &response{status: resp.StatusCode, headers: resp.Header, body: body}

	extracted := make(map[string]string, len(nr.Request.Extract))
	for name, tmpl := range nr.Request.Extract {
		// Extract templates may reference both the response and the scope.
		val, err := interpolate.Interpolate(tmpl, func(n string) (string, bool) {
			if v, ok := rx.resolve(n); ok {
				return v, true
			}
			v, ok := scope[n]
			return v, ok
		})
		if err != nil {
			res.Err = fmt.Errorf("extract %q: %w", name, err)
			return res, nil
		}
		extracted[name] = val
	}

	if e := nr.Request.Expect; e != nil {
		res.Failures = evaluateExpect(e, resp.StatusCode, body, res.Duration)
	}
	return res, extracted
}

// expandVars expands ${env:} and {{var}} references in request-level vars
// against scope.
func expandVars(vars map[string]string, scope map[string]string) (map[string]string, error) {
	if len(vars) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(vars))
	for k, v := range vars {
		expanded, err := interpolate.Interpolate(v, func(name string) (string, bool) {
			s, ok := scope[name]
			return s, ok
		})
		if err != nil {
			return nil, fmt.Errorf("expand var %q: %w", k, err)
		}
		out[k] = expanded
	}
	return out, nil
}

func mergeMaps(base, over map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

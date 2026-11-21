// Package web serves harpoon's browser UI and a small local JSON API from
// the same binary: the frontend is embedded with //go:embed, so the tool
// stays a single static executable with no runtime dependencies.
package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/runner"
)

//go:embed static
var staticFS embed.FS

// Server holds the interactive session state behind the web UI: one shared
// runner.Session (so variable chains work across runs), the latest result
// per request, and the run history.
type Server struct {
	coll    *config.Collection
	sess    *runner.Session
	profile map[string]string
	cliVars map[string]string

	mu      sync.Mutex
	results map[string]runner.Result
	history []historyEntry
}

type historyEntry struct {
	At      time.Time   `json:"at"`
	Results []apiResult `json:"results"`
}

// New builds a server for a collection.
func New(coll *config.Collection, profile, cliVars map[string]string) *Server {
	return &Server{
		coll:    coll,
		sess:    runner.NewSession(),
		profile: profile,
		cliVars: cliVars,
		results: make(map[string]runner.Result),
	}
}

// Handler assembles the routes: the JSON API under /api, the embedded SPA
// everywhere else.
func (s *Server) Handler() http.Handler {
	staticRoot, _ := fs.Sub(staticFS, "static")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/collection", s.handleCollection)
	mux.HandleFunc("POST /api/run", s.handleRun)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.Handle("GET /", http.FileServerFS(staticRoot))
	return mux
}

// --- JSON API DTOs -------------------------------------------------------

type apiRequest struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

type apiCollection struct {
	Name     string       `json:"name"`
	Requests []apiRequest `json:"requests"`
}

type apiCheck struct {
	Kind   string `json:"kind"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type apiResult struct {
	Name       string              `json:"name"`
	Method     string              `json:"method"`
	URL        string              `json:"url"`
	Status     int                 `json:"status"`
	DurationMS float64             `json:"duration_ms"`
	Err        string              `json:"err,omitempty"`
	Failures   []string            `json:"failures,omitempty"`
	Checks     []apiCheck          `json:"checks,omitempty"`
	Extracted  map[string]string   `json:"extracted,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string              `json:"body,omitempty"`
	BodyJSON   bool                `json:"body_json"`
}

// --- handlers ------------------------------------------------------------

func (s *Server) handleCollection(w http.ResponseWriter, _ *http.Request) {
	out := apiCollection{Name: s.coll.Name, Requests: make([]apiRequest, 0, len(s.coll.Requests))}
	for _, nr := range s.coll.Requests {
		out.Requests = append(out.Requests, apiRequest{Name: nr.Name, Method: nr.Request.Method, URL: nr.Request.URL})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRun executes requests in the session and records history. Runs are
// serialized: the shared session's variable chain must stay coherent.
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Requests []string `json:"requests"`
		All      bool     `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	var target *config.Collection
	switch {
	case body.All:
		target = s.coll
	case len(body.Requests) == 0:
		httpError(w, http.StatusBadRequest, `body must be {"all":true} or {"requests":["name"]}`)
		return
	default:
		var err error
		target, err = config.Select(s.coll, body.Requests)
		if err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	s.mu.Lock()
	results := s.sess.Run(target, s.profile, runner.Options{CLIVars: s.cliVars})
	entry := historyEntry{At: time.Now(), Results: make([]apiResult, 0, len(results))}
	for _, res := range results {
		s.results[res.Name] = res
		entry.Results = append(entry.Results, toAPIResult(res))
	}
	s.history = append(s.history, entry)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"results": entry.Results})
}

func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]historyEntry, 0, len(s.history))
	for i := len(s.history) - 1; i >= 0; i-- { // newest first
		out = append(out, s.history[i])
	}
	writeJSON(w, http.StatusOK, out)
}

// --- helpers -------------------------------------------------------------

func toAPIResult(r runner.Result) apiResult {
	out := apiResult{
		Name:       r.Name,
		Method:     r.Method,
		URL:        r.URL,
		Status:     r.Status,
		DurationMS: float64(r.Duration.Microseconds()) / 1000,
		Failures:   r.Failures,
		Extracted:  r.Extracted,
		Headers:    r.Headers,
		Body:       string(r.Body),
		BodyJSON:   len(r.Body) > 0 && json.Valid(r.Body),
	}
	if r.Err != nil {
		out.Err = r.Err.Error()
	}
	for _, c := range r.Checks {
		out.Checks = append(out.Checks, apiCheck{Kind: c.Kind, OK: c.OK, Detail: c.Detail})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

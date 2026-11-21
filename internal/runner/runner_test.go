package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihai-corder/harpoon/internal/config"
)

// newTestServer starts an HTTP backend that mimics a login + authenticated
// listing flow, so collections can be exercised end to end.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth/login":
			var creds struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if creds.Username != "demo" || creds.Password != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"token": "tok-123"}`)

		case r.URL.Path == "/users":
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Request-Id", "req-42")
			fmt.Fprint(w, `{"users": [{"id": 1, "name": "alice"}]}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error": "not found"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeCollection(t *testing.T, content string) *config.Collection {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "collection.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	coll, err := config.Load(path)
	require.NoError(t, err)
	return coll
}

func TestRunChainedRequests(t *testing.T) {
	srv := newTestServer(t)

	coll := writeCollection(t, fmt.Sprintf(`
name: integration
requests:
  login:
    method: POST
    url: %[1]s/auth/login
    body:
      username: demo
      password: secret
    extract:
      token: "{{response.body.token}}"
      login_status: "{{response.status}}"
  list-users:
    method: GET
    url: %[1]s/users
    headers:
      Authorization: "Bearer {{token}}"
      X-Trace: "{{login_status}}"
    expect:
      status: 200
      contains: "alice"
    extract:
      first_user: "{{response.body.users.0.name}}"
      request_id: "{{response.headers.X-Request-Id}}"
`, srv.URL))

	results := Run(coll, nil, Options{})
	require.Len(t, results, 2)

	login := results[0]
	assert.True(t, login.Passed(), "login: err=%v failures=%v", login.Err, login.Failures)
	assert.Equal(t, 200, login.Status)
	assert.Equal(t, "POST", login.Method)
	assert.True(t, login.Duration > 0)

	list := results[1]
	assert.True(t, list.Passed(), "list-users: err=%v failures=%v", list.Err, list.Failures)
	assert.Equal(t, 200, list.Status)
	assert.Equal(t, "GET", list.Method)
}

// TestRunExtractionFeedsLaterRequests pins down the chaining contract: the
// second request only succeeds because the first one extracted a valid token.
func TestRunExtractionFeedsLaterRequests(t *testing.T) {
	srv := newTestServer(t)

	coll := writeCollection(t, fmt.Sprintf(`
requests:
  login:
    method: POST
    url: %[1]s/auth/login
    body:
      username: demo
      password: secret
    extract:
      token: "{{response.body.token}}"
  list-users:
    method: GET
    url: %[1]s/users
    headers:
      Authorization: "Bearer {{token}}"
    expect:
      status: 200
`, srv.URL))

	results := Run(coll, nil, Options{})
	for _, r := range results {
		assert.True(t, r.Passed(), "%s: err=%v failures=%v", r.Name, r.Err, r.Failures)
	}
}

func TestRunBadTokenFailsChain(t *testing.T) {
	srv := newTestServer(t)

	coll := writeCollection(t, fmt.Sprintf(`
requests:
  login:
    method: POST
    url: %[1]s/auth/login
    body:
      username: demo
      password: wrong-password
    extract:
      token: "{{response.body.token}}"
  list-users:
    method: GET
    url: %[1]s/users
    headers:
      Authorization: "Bearer {{token}}"
    expect:
      status: 200
`, srv.URL))

	results := Run(coll, nil, Options{})
	require.Len(t, results, 2)
	// Login returns 401 with an empty body, so the token extraction itself
	// fails — which is the behavior we want: a login that produces no token
	// must not look like a pass.
	assert.False(t, results[0].Passed())
	require.Error(t, results[0].Err)
	assert.Contains(t, results[0].Err.Error(), "extract")

	assert.False(t, results[1].Passed(), "expected the auth check to fail the second request")
}

func TestRunAssertionFailures(t *testing.T) {
	srv := newTestServer(t)

	coll := writeCollection(t, fmt.Sprintf(`
requests:
  users:
    method: GET
    url: %[1]s/users
    headers:
      Authorization: "Bearer tok-123"
    expect:
      status: 404
      contains: "nothing-like-this"
      max_ms: 5000
`, srv.URL))

	results := Run(coll, nil, Options{})
	require.Len(t, results, 1)
	res := results[0]
	assert.False(t, res.Passed())
	require.Len(t, res.Failures, 2)
	assert.Contains(t, res.Failures[0], "expected status 404, got 200")
	assert.Contains(t, res.Failures[1], `body does not contain "nothing-like-this"`)
}

func TestRunWithoutExpectPasses(t *testing.T) {
	srv := newTestServer(t)

	coll := writeCollection(t, fmt.Sprintf(`
requests:
  users:
    method: GET
    url: %[1]s/users
    headers:
      Authorization: "Bearer tok-123"
`, srv.URL))

	results := Run(coll, nil, Options{})
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed())
}

func TestRunUnknownVariable(t *testing.T) {
	srv := newTestServer(t)

	coll := writeCollection(t, fmt.Sprintf(`
requests:
  users:
    method: GET
    url: %[1]s/users?for={{nope}}
`, srv.URL))

	results := Run(coll, nil, Options{})
	require.Len(t, results, 1)
	res := results[0]
	assert.False(t, res.Passed())
	require.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "{{nope}}")
	assert.Zero(t, res.Status)
}

func TestRunVariablePriority(t *testing.T) {
	var called string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = r.Header.Get("X-Who")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	coll := writeCollection(t, fmt.Sprintf(`
vars:
  who: collection
requests:
  ping:
    url: %[1]s
    vars:
      who: request
    headers:
      X-Who: "{{who}}"
`, srv.URL))

	// CLI vars beat request-level vars.
	Run(coll, nil, Options{CLIVars: map[string]string{"who": "cli"}})
	assert.Equal(t, "cli", called)

	// Without CLI vars, request-level vars beat collection vars.
	Run(coll, nil, Options{})
	assert.Equal(t, "request", called)
}

func TestRunProfileAndRequestVars(t *testing.T) {
	var called string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = r.Header.Get("X-Who")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	coll := writeCollection(t, fmt.Sprintf(`
requests:
  ping:
    url: %[1]s
    headers:
      X-Who: "{{who}}"
`, srv.URL))

	Run(coll, map[string]string{"who": "profile"}, Options{})
	assert.Equal(t, "profile", called)
}

func TestMaxMSAssertion(t *testing.T) {
	// A deliberately slow backend makes the 1ms budget deterministically
	// exceeded; localhost round trips are otherwise faster than 1ms.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer slow.Close()

	coll := writeCollection(t, fmt.Sprintf(`
requests:
  slow:
    url: %[1]s
    expect:
      max_ms: 1
`, slow.URL))

	results := Run(coll, nil, Options{})
	require.Len(t, results, 1)
	assert.False(t, results[0].Passed())
	require.NotEmpty(t, results[0].Failures)
	assert.Contains(t, results[0].Failures[0], "max allowed 1ms")
}

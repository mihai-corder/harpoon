package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihai-corder/harpoon/internal/config"
)

// newBackend mimics a login + authenticated listing API, like the runner
// tests, so the web layer can be exercised end to end.
func newBackend(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth/login":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"token": "tok-123"}`)
		case r.URL.Path == "/users":
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"users": [{"id": 1, "name": "alice"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newWebServer(t *testing.T, yaml string) (*Server, *httptest.Server) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "collection.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o644))

	coll, err := config.Load(path)
	require.NoError(t, err)

	s := New(coll, nil, nil)
	api := httptest.NewServer(s.Handler())
	t.Cleanup(api.Close)
	return s, api
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	res, err := http.Get(url)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, json.NewDecoder(res.Body).Decode(out))
}

func TestServeCollectionAndStatic(t *testing.T) {
	_, api := newWebServer(t, `
name: demo
requests:
  second:
    url: http://example.com/2
  first:
    url: http://example.com/1
`)

	var coll apiCollection
	getJSON(t, api.URL+"/api/collection", &coll)
	assert.Equal(t, "demo", coll.Name)
	require.Len(t, coll.Requests, 2)
	assert.Equal(t, "second", coll.Requests[0].Name) // file order preserved
	assert.Equal(t, "first", coll.Requests[1].Name)

	res, err := http.Get(api.URL + "/")
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, string(body), "harpoon")

	for _, asset := range []string{"/app.css", "/app.js"} {
		res, err := http.Get(api.URL + asset)
		require.NoError(t, err)
		res.Body.Close()
		assert.Equal(t, http.StatusOK, res.StatusCode, asset)
	}
}

func TestServeRunAllAndHistory(t *testing.T) {
	backend := newBackend(t)
	_, api := newWebServer(t, fmt.Sprintf(`
name: demo
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
`, backend.URL))

	var runRes struct {
		Results []apiResult `json:"results"`
	}
	res, err := http.Post(api.URL+"/api/run", "application/json", strings.NewReader(`{"all":true}`))
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&runRes))

	require.Len(t, runRes.Results, 2)
	assert.Equal(t, 200, runRes.Results[0].Status)
	assert.Equal(t, "tok-123", runRes.Results[0].Extracted["token"])
	assert.True(t, runRes.Results[0].BodyJSON)
	assert.Equal(t, 200, runRes.Results[1].Status)

	var history []historyEntry
	getJSON(t, api.URL+"/api/history", &history)
	require.Len(t, history, 1)
	require.Len(t, history[0].Results, 2)
}

// TestServeSessionCarriesAcrossRuns is the web twin of the runner session
// test: run login, then run the authenticated request alone — the token
// must still be in the shared session.
func TestServeSessionCarriesAcrossRuns(t *testing.T) {
	backend := newBackend(t)
	_, api := newWebServer(t, fmt.Sprintf(`
name: demo
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
`, backend.URL))

	run := func(body string) []apiResult {
		res, err := http.Post(api.URL+"/api/run", "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, http.StatusOK, res.StatusCode)
		var out struct {
			Results []apiResult `json:"results"`
		}
		require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
		return out.Results
	}

	login := run(`{"requests":["login"]}`)
	require.Len(t, login, 1)
	assert.False(t, login[0].Failed())

	users := run(`{"requests":["list-users"]}`)
	require.Len(t, users, 1)
	assert.False(t, users[0].Failed(), "session should carry the token")
}

func TestServeRunRejectsUnknownAndEmpty(t *testing.T) {
	_, api := newWebServer(t, `
requests:
  a:
    url: http://example.com
`)

	res, err := http.Post(api.URL+"/api/run", "application/json", strings.NewReader(`{"requests":["nope"]}`))
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)

	res2, err := http.Post(api.URL+"/api/run", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer res2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, res2.StatusCode)
}

// Failed is a small helper matching runner.Result.Passed for the DTO.
func (r apiResult) Failed() bool {
	return r.Err != "" || len(r.Failures) > 0
}

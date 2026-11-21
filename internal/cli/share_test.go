package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShareCmdUploadsAndPrintsURL(t *testing.T) {
	var gotHTML string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/reports", r.URL.Path)
		var body struct {
			HTML string `json:"html"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotHTML = body.HTML
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"Ab3xK9mQ","url":"https://s.test/r/Ab3xK9mQ","delete_token":"tok","expires_at":"2027-02-19T00:00:00Z"}`)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "report.html")
	require.NoError(t, os.WriteFile(path, []byte("<!doctype html><html><body>report</body></html>"), 0o644))

	root := NewRootCmd()
	root.SetArgs([]string{"share", path, "--server", srv.URL})
	require.NoError(t, root.Execute())

	assert.Contains(t, gotHTML, "report")
}

func TestShareCmdMissingFile(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"share", filepath.Join(t.TempDir(), "nope.html"), "--server", "http://127.0.0.1:1"})
	require.Error(t, root.Execute())
}

func TestShareCmdServerErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limit exceeded"}`)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "report.html")
	require.NoError(t, os.WriteFile(path, []byte("<html></html>"), 0o644))

	root := NewRootCmd()
	root.SetArgs([]string{"share", path, "--server", srv.URL})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

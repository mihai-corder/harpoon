package share

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadSuccess(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/api/reports", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"Ab3xK9mQ","url":"https://s.test/r/Ab3xK9mQ","delete_token":"tok","expires_at":"2027-02-19T00:00:00Z"}`)
	}))
	defer srv.Close()

	client := &Client{Server: srv.URL}
	out, err := client.Upload([]byte("<!doctype html><html><body>ok</body></html>"), "demo")
	require.NoError(t, err)

	assert.Equal(t, "Ab3xK9mQ", out.ID)
	assert.Equal(t, "https://s.test/r/Ab3xK9mQ", out.URL)
	assert.Equal(t, "demo", receivedBody["collection"])
}

func TestUploadServerErrors(t *testing.T) {
	cases := map[string]struct {
		status  int
		body    string
		wantErr string
	}{
		"too large": {http.StatusRequestEntityTooLarge, `{"error":"html exceeds the 1048576 byte limit"}`, "html exceeds"},
		"rate hit":  {http.StatusTooManyRequests, `{"error":"rate limit exceeded"}`, "rate limit"},
		"bad json":  {http.StatusBadRequest, `{"error":"body must be JSON"}`, "body must be JSON"},
		"plain":     {http.StatusInternalServerError, `oops`, "share server returned 500: oops"},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()

			client := &Client{Server: srv.URL}
			_, err := client.Upload([]byte("<html></html>"), "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestUploadUnreachableServer(t *testing.T) {
	client := &Client{Server: "http://127.0.0.1:1"}
	_, err := client.Upload([]byte("<html></html>"), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "share server unreachable")
}

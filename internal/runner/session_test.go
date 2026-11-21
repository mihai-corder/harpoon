package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionCarriesExtractionAcrossRuns pins down the TUI contract: run
// login once, then any authenticated request later — on its own — and the
// session still has the token.
func TestSessionCarriesExtractionAcrossRuns(t *testing.T) {
	srv := newTestServer(t)

	full := writeCollection(t, fmt.Sprintf(`
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

	sess := NewSession()

	loginOnly := *full
	loginOnly.Requests = full.Requests[:1]
	res := sess.Run(&loginOnly, nil, Options{})
	require.Len(t, res, 1)
	assert.True(t, res[0].Passed())

	authOnly := *full
	authOnly.Requests = full.Requests[1:2]
	res = sess.Run(&authOnly, nil, Options{})
	require.Len(t, res, 1)
	assert.True(t, res[0].Passed(), "session should carry the token: err=%v failures=%v",
		res[0].Err, res[0].Failures)
}

// TestRunIsIsolated verifies the package-level convenience wrapper does not
// leak extractions between separate invocations.
func TestRunIsIsolated(t *testing.T) {
	srv := newTestServer(t)

	loginOnly := writeCollection(t, fmt.Sprintf(`
requests:
  login:
    method: POST
    url: %[1]s/auth/login
    body:
      username: demo
      password: secret
    extract:
      token: "{{response.body.token}}"
`, srv.URL))

	authOnly := writeCollection(t, fmt.Sprintf(`
requests:
  list-users:
    method: GET
    url: %[1]s/users
    headers:
      Authorization: "Bearer {{token}}"
    expect:
      status: 200
`, srv.URL))

	res := Run(loginOnly, nil, Options{})
	require.Len(t, res, 1)
	assert.True(t, res[0].Passed())

	res = Run(authOnly, nil, Options{})
	require.Len(t, res, 1)
	assert.False(t, res[0].Passed(), "a fresh Run must not inherit the previous token")
}

// TestResultExposesResponseForInspection covers the fields the TUI renders.
func TestResultExposesResponseForInspection(t *testing.T) {
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
      contains: "alice"
`, srv.URL))

	results := Run(coll, nil, Options{})
	require.Len(t, results, 2)

	login := results[0]
	assert.Equal(t, "tok-123", login.Extracted["token"])
	assert.NotEmpty(t, login.Body)
	assert.NotNil(t, login.Headers)

	list := results[1]
	require.Len(t, list.Checks, 2)
	assert.True(t, list.Checks[0].OK)
	assert.Equal(t, "status", list.Checks[0].Kind)
	assert.Equal(t, "status is 200", list.Checks[0].Detail)
	assert.True(t, list.Checks[1].OK)
	assert.Equal(t, "contains", list.Checks[1].Kind)
	assert.Contains(t, string(list.Body), "alice")
}

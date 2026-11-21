package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadPreservesRequestOrder(t *testing.T) {
	path := write(t, t.TempDir(), "collection.yaml", `
name: demo
requests:
  zebra:
    url: http://example.com/z
  alpha:
    url: http://example.com/a
  mike:
    url: http://example.com/m
`)

	coll, err := Load(path)
	require.NoError(t, err)

	names := make([]string, 0, len(coll.Requests))
	for _, nr := range coll.Requests {
		names = append(names, nr.Name)
	}
	assert.Equal(t, []string{"zebra", "alpha", "mike"}, names)
}

func TestLoadDefaultsAndNormalization(t *testing.T) {
	path := write(t, t.TempDir(), "collection.yaml", `
requests:
  ping:
    url: http://example.com
    method: post
`)
	coll, err := Load(path)
	require.NoError(t, err)

	ping := coll.Requests[0].Request
	assert.Equal(t, "POST", ping.Method)

	coll, err = Load(path) // name falls back to the file name
	require.NoError(t, err)
	assert.Equal(t, "collection", coll.Name)
}

func TestLoadRejectsBadRequests(t *testing.T) {
	cases := map[string]string{
		"missing url": `
requests:
  broken:
    method: GET
`,
		"unknown method": `
requests:
  broken:
    method: FETCH
    url: http://example.com
`,
		"bad status": `
requests:
  broken:
    url: http://example.com
    expect:
      status: 42
`,
		"negative max_ms": `
requests:
  broken:
    url: http://example.com
    expect:
      max_ms: -1
`,
		"duplicate name": `
requests:
  dup:
    url: http://example.com/1
  dup:
    url: http://example.com/2
`,
		"no requests": `
name: empty
`,
	}

	for label, yaml := range cases {
		t.Run(label, func(t *testing.T) {
			path := write(t, t.TempDir(), "collection.yaml", yaml)
			_, err := Load(path)
			require.Error(t, err)
		})
	}
}

func TestLoadExpandsEnvRefsInVars(t *testing.T) {
	t.Setenv("HARPOON_TEST_PW", "s3cret")
	path := write(t, t.TempDir(), "collection.yaml", `
vars:
  password: ${env:HARPOON_TEST_PW}
requests:
  login:
    url: http://example.com
`)
	coll, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "s3cret", coll.Vars["password"])
}

func TestLoadMissingEnvRef(t *testing.T) {
	t.Setenv("HARPOON_TEST_PW", "")
	os.Unsetenv("HARPOON_TEST_PW")

	path := write(t, t.TempDir(), "collection.yaml", `
vars:
  password: ${env:HARPOON_TEST_PW}
requests:
  login:
    url: http://example.com
`)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "${env:HARPOON_TEST_PW}")
}

func TestLoadProfile(t *testing.T) {
	dir := t.TempDir()
	collection := write(t, dir, "collection.yaml", "requests:\n  a:\n    url: http://example.com\n")
	write(t, dir, "harpoon.dev.yaml", "base_url: http://dev.example.com\n")

	vars, err := LoadProfile(collection, "dev")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"base_url": "http://dev.example.com"}, vars)

	vars, err = LoadProfile(collection, "")
	require.NoError(t, err)
	assert.Empty(t, vars)

	_, err = LoadProfile(collection, "staging")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staging")
}

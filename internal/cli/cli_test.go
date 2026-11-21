package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCollection(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "collection.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestValidateCmdValid(t *testing.T) {
	path := writeCollection(t, `
vars:
  base_url: http://example.com
requests:
  ping:
    url: "{{base_url}}/ping"
    expect:
      status: 200
`)

	root := NewRootCmd()
	root.SetArgs([]string{"validate", path})
	require.NoError(t, root.Execute())
}

func TestValidateCmdWarnsOnUndefinedVar(t *testing.T) {
	path := writeCollection(t, `
requests:
  ping:
    url: "{{not_defined_anywhere}}/ping"
`)

	root := NewRootCmd()
	root.SetArgs([]string{"validate", path})
	require.NoError(t, root.Execute()) // warnings do not fail the command
}

func TestValidateCmdRejectsStructuralErrors(t *testing.T) {
	path := writeCollection(t, `
requests:
  broken:
    method: FETCH
    url: http://example.com
`)

	root := NewRootCmd()
	root.SetArgs([]string{"validate", path})
	require.Error(t, root.Execute())
}

func TestListCmd(t *testing.T) {
	path := writeCollection(t, `
requests:
  second:
    method: POST
    url: http://example.com/second
  first:
    url: http://example.com/first
`)

	root := NewRootCmd()
	root.SetArgs([]string{"list", path})
	require.NoError(t, root.Execute())
}

func TestParseVars(t *testing.T) {
	vars, err := parseVars([]string{"a=1", "b=two=parts"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "two=parts"}, vars)

	_, err = parseVars([]string{"broken"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KEY=VALUE")
}

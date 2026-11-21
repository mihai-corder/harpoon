package interpolate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lookup(vars map[string]string) Resolver {
	return func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	}
}

func TestInterpolateSimple(t *testing.T) {
	out, err := Interpolate("Bearer {{token}}", lookup(map[string]string{"token": "abc"}))
	require.NoError(t, err)
	assert.Equal(t, "Bearer abc", out)
}

func TestInterpolateMultipleAndPassthrough(t *testing.T) {
	vars := map[string]string{"a": "1", "b": "2"}

	out, err := Interpolate("{{a}}-{{b}}-{{a}}", lookup(vars))
	require.NoError(t, err)
	assert.Equal(t, "1-2-1", out)

	out, err = Interpolate("no placeholders here", lookup(vars))
	require.NoError(t, err)
	assert.Equal(t, "no placeholders here", out)

	out, err = Interpolate("", lookup(vars))
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestInterpolateTrimsSpaces(t *testing.T) {
	out, err := Interpolate("hi {{ name }}", lookup(map[string]string{"name": "world"}))
	require.NoError(t, err)
	assert.Equal(t, "hi world", out)
}

func TestInterpolateUnknownVariable(t *testing.T) {
	_, err := Interpolate("GET {{missing}}/x", lookup(map[string]string{"a": "1"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `{{missing}}`)
}

func TestInterpolateEmptyPlaceholder(t *testing.T) {
	_, err := Interpolate("{{ }}", lookup(nil))
	require.Error(t, err)
}

func TestInterpolateEnvRef(t *testing.T) {
	t.Setenv("HARPOON_TEST_VAR", "envval")

	out, err := Interpolate("x=${env:HARPOON_TEST_VAR} y={{a}}", lookup(map[string]string{"a": "1"}))
	require.NoError(t, err)
	assert.Equal(t, "x=envval y=1", out)
}

func TestInterpolateMissingEnvRef(t *testing.T) {
	t.Setenv("HARPOON_TEST_VAR", "") // ensure the name is not in the environment
	os.Unsetenv("HARPOON_TEST_VAR")

	_, err := Interpolate("${env:HARPOON_TEST_VAR}", lookup(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "${env:HARPOON_TEST_VAR}")
}

func TestExpandEnvRefs(t *testing.T) {
	t.Setenv("HARPOON_TEST_VAR", "secret")

	out, err := ExpandEnvRefs("{{keep}} ${env:HARPOON_TEST_VAR}")
	require.NoError(t, err)
	assert.Equal(t, "{{keep}} secret", out)

	_, err = ExpandEnvRefs("${env:HARPOON_TEST_MISSING}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "${env:HARPOON_TEST_MISSING}")
}

func TestPlaceholders(t *testing.T) {
	got := Placeholders("u={{user}} h={{response.headers.x-request-id}} ${env:FOO} bare={{ response.status }}")
	assert.Equal(t, []string{"user", "response.headers.x-request-id", "response.status"}, got)
}

func TestWalkStrings(t *testing.T) {
	in := map[string]any{
		"name": "{{user}}",
		"nested": map[string]any{
			"list": []any{"{{a}}", 42, true},
		},
		"count": 7,
	}

	out, err := WalkStrings(in, func(s string) (string, error) {
		return "<" + s + ">", nil
	})
	require.NoError(t, err)

	m := out.(map[string]any)
	assert.Equal(t, "<{{user}}>", m["name"])
	nested := m["nested"].(map[string]any)
	list := nested["list"].([]any)
	assert.Equal(t, "<{{a}}>", list[0])
	assert.Equal(t, 42, list[1])
	assert.Equal(t, true, list[2])
	assert.Equal(t, 7, m["count"])
}

func TestWalkStringsPropagatesError(t *testing.T) {
	_, err := WalkStrings(map[string]any{"k": "{{bad}}"}, func(string) (string, error) {
		return "", os.ErrPermission
	})
	require.Error(t, err)
}

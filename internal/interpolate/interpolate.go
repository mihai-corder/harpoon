// Package interpolate expands variable references inside request templates.
//
// Two syntaxes are supported:
//
//	{{name}}      — looked up via the provided Resolver (collection, profile,
//	                request-level and CLI variables, or response.* references)
//	${env:NAME}   — read from the process environment, never stored on disk
//
// Placeholders are expanded in a single pass; nested placeholders are not
// supported.
package interpolate

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Resolver looks up the value of a {{name}} placeholder. The second return
// value reports whether the name was found.
type Resolver func(name string) (string, bool)

var placeholder = regexp.MustCompile(`\{\{([^{}]*)\}\}|\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// Interpolate expands every placeholder in s. Unknown {{name}} references or
// missing ${env:NAME} variables produce an error listing the offenders.
func Interpolate(s string, resolve Resolver) (string, error) {
	var missing []string
	out := placeholder.ReplaceAllStringFunc(s, func(match string) string {
		if name, ok := envRef(match); ok {
			v, found := os.LookupEnv(name)
			if !found {
				missing = append(missing, match)
				return ""
			}
			return v
		}
		name := strings.TrimSpace(match[2 : len(match)-2])
		if name == "" {
			missing = append(missing, match)
			return ""
		}
		v, found := resolve(name)
		if !found {
			missing = append(missing, match)
			return ""
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("undefined placeholder(s): %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// ExpandEnvRefs expands only ${env:NAME} references, leaving {{name}} intact.
// It is used for static variable values in collection and profile files,
// where {{name}} references are not allowed.
func ExpandEnvRefs(s string) (string, error) {
	var missing []string
	out := envPattern.ReplaceAllStringFunc(s, func(match string) string {
		name, _ := envRef(match)
		v, found := os.LookupEnv(name)
		if !found {
			missing = append(missing, match)
			return ""
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("undefined environment variable(s): %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// Placeholders returns every {{name}} reference in s (without braces), in
// order of appearance. ${env:...} references are not included.
func Placeholders(s string) []string {
	var out []string
	for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
		if !strings.HasPrefix(m[0], "{{") {
			continue
		}
		if name := strings.TrimSpace(m[1]); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// WalkStrings applies fn to every string in a YAML-decoded structure
// (nested maps, slices and scalars) and returns a rebuilt structure.
// Non-string scalars are passed through unchanged.
func WalkStrings(v any, fn func(string) (string, error)) (any, error) {
	switch t := v.(type) {
	case string:
		return fn(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			nv, err := WalkStrings(val, fn)
			if err != nil {
				return nil, err
			}
			out[k] = nv
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			nv, err := WalkStrings(val, fn)
			if err != nil {
				return nil, err
			}
			out[fmt.Sprint(k)] = nv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			nv, err := WalkStrings(val, fn)
			if err != nil {
				return nil, err
			}
			out[i] = nv
		}
		return out, nil
	default:
		return v, nil
	}
}

// envRef reports whether match is a ${env:NAME} reference, returning NAME.
func envRef(match string) (string, bool) {
	const prefix = "${env:"
	if !strings.HasPrefix(match, prefix) {
		return "", false
	}
	return match[len(prefix) : len(match)-1], true
}

var envPattern = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mihai-corder/harpoon/internal/interpolate"
	"gopkg.in/yaml.v3"
)

var methods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

// fileCollection mirrors the on-disk layout. Requests is kept as a raw node
// so the order of the mapping can be preserved.
type fileCollection struct {
	Name     string            `yaml:"name"`
	Vars     map[string]string `yaml:"vars,omitempty"`
	Requests yaml.Node         `yaml:"requests"`
}

// Load reads and validates a collection file. ${env:NAME} references inside
// file-level vars are expanded immediately; {{name}} references are left for
// the runner.
func Load(path string) (*Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fc fileCollection
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	c := &Collection{Name: fc.Name, Vars: map[string]string{}}
	if c.Name == "" {
		base := filepath.Base(path)
		c.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	for k, v := range fc.Vars {
		expanded, err := interpolate.ExpandEnvRefs(v)
		if err != nil {
			return nil, fmt.Errorf("%s: var %q: %w", path, k, err)
		}
		c.Vars[k] = expanded
	}

	if fc.Requests.Kind != yaml.MappingNode {
		if fc.Requests.Kind == 0 {
			return nil, fmt.Errorf("%s: no requests defined", path)
		}
		return nil, fmt.Errorf(`%s: "requests" must be a mapping of name -> request`, path)
	}

	content := fc.Requests.Content
	seen := map[string]bool{}
	for i := 0; i+1 < len(content); i += 2 {
		keyNode, valNode := content[i], content[i+1]
		name := keyNode.Value
		if name == "" {
			return nil, fmt.Errorf("%s: request name must not be empty", path)
		}
		if seen[name] {
			return nil, fmt.Errorf("%s: duplicate request name %q", path, name)
		}
		seen[name] = true

		var req Request
		if err := valNode.Decode(&req); err != nil {
			return nil, fmt.Errorf("%s: request %q: %w", path, name, err)
		}
		if err := normalizeRequest(&req); err != nil {
			return nil, fmt.Errorf("%s: request %q: %w", path, name, err)
		}
		c.Requests = append(c.Requests, NamedRequest{Name: name, Request: req})
	}
	if len(c.Requests) == 0 {
		return nil, fmt.Errorf("%s: no requests defined", path)
	}
	return c, nil
}

// LoadProfile reads harpoon.<env>.yaml from the collection's directory.
// An empty env name returns an empty profile and never touches the disk.
func LoadProfile(collectionPath, env string) (map[string]string, error) {
	if env == "" {
		return map[string]string{}, nil
	}
	path := filepath.Join(filepath.Dir(collectionPath), "harpoon."+env+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("env profile %q not found (looked for %s)", env, path)
		}
		return nil, err
	}

	var vars map[string]string
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	profile := map[string]string{}
	for k, v := range vars {
		expanded, err := interpolate.ExpandEnvRefs(v)
		if err != nil {
			return nil, fmt.Errorf("%s: var %q: %w", path, k, err)
		}
		profile[k] = expanded
	}
	return profile, nil
}

func normalizeRequest(r *Request) error {
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if r.Method == "" {
		r.Method = "GET"
	}
	if !methods[r.Method] {
		return fmt.Errorf("unsupported method %q (allowed: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)", r.Method)
	}
	if strings.TrimSpace(r.URL) == "" {
		return errors.New("url is required")
	}
	if r.Expect == nil {
		return nil
	}
	if r.Expect.Status != 0 && (r.Expect.Status < 100 || r.Expect.Status > 599) {
		return fmt.Errorf("expect.status %d is out of range 100-599", r.Expect.Status)
	}
	if r.Expect.MaxMS < 0 {
		return errors.New("expect.max_ms must be >= 0")
	}
	return nil
}

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestActionYamlIsValid guards the composite GitHub Action: it must stay
// parseable YAML with the structure GitHub expects.
func TestActionYamlIsValid(t *testing.T) {
	data, err := os.ReadFile("action.yml")
	require.NoError(t, err)

	var doc struct {
		Name string `yaml:"name"`
		Runs struct {
			Using string `yaml:"using"`
			Steps []struct {
				Name  string `yaml:"name,omitempty"`
				Uses  string `yaml:"uses,omitempty"`
				Shell string `yaml:"shell,omitempty"`
				ID    string `yaml:"id,omitempty"`
			} `yaml:"steps"`
		} `yaml:"runs"`
		Inputs map[string]struct {
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
			Default     string `yaml:"default"`
		} `yaml:"inputs"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))

	assert.NotEmpty(t, doc.Name)
	assert.Equal(t, "composite", doc.Runs.Using)
	assert.NotEmpty(t, doc.Runs.Steps)

	for _, key := range []string{"collection", "comment", "token"} {
		assert.Contains(t, doc.Inputs, key)
	}

	stepNames := make([]string, 0, len(doc.Runs.Steps))
	for _, s := range doc.Runs.Steps {
		stepNames = append(stepNames, s.Name)
	}
	assert.Contains(t, stepNames, "Run harpoon ci")
	assert.Contains(t, stepNames, "Post PR comment")
}

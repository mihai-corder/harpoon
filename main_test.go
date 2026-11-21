package main

import (
	"os"
	"path/filepath"
	"strings"
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

	for _, key := range []string{"collection", "comment", "token", "share", "share-server"} {
		assert.Contains(t, doc.Inputs, key)
	}

	stepNames := make([]string, 0, len(doc.Runs.Steps))
	for _, s := range doc.Runs.Steps {
		stepNames = append(stepNames, s.Name)
	}
	assert.Contains(t, stepNames, "Run harpoon ci")
	assert.Contains(t, stepNames, "Share HTML report")
	assert.Contains(t, stepNames, "Post PR comment")
}

// TestDogfoodWorkflowIsValid guards the dogfooding workflow: it must keep
// using the official action against the bundled collection.
func TestDogfoodWorkflowIsValid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(".github", "workflows", "dogfood.yml"))
	require.NoError(t, err)

	var doc struct {
		Name string `yaml:"name"`
		Jobs map[string]struct {
			Steps []struct {
				Uses string `yaml:"uses"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))

	assert.Equal(t, "Dogfood", doc.Name)
	job, ok := doc.Jobs["smoke"]
	require.True(t, ok, "smoke job must exist")

	usesAction := false
	for _, s := range job.Steps {
		if strings.Contains(s.Uses, "mihai-corder/harpoon@") {
			usesAction = true
		}
	}
	assert.True(t, usesAction, "the workflow must run through the official action")
}

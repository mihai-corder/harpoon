// Package cli implements harpoon's command-line interface.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// version is the default for development builds; release builds can inject
// the real version via -ldflags.
var version = "0.5.0"

// NewRootCmd assembles the harpoon command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "harpoon",
		Short: "Terminal-first API testing: collections as code, from TUI to CI.",
		Long: `Harpoon is a terminal-first API testing tool.

Define request collections in plain YAML files you commit to git,
chain variables between requests, and reuse the exact same files
as smoke tests in CI.`,
		Version:      version,
		SilenceUsage: true,
	}
	root.AddCommand(newRunCmd(), newCiCmd(), newShareCmd(), newValidateCmd(), newListCmd(), newTuiCmd(), newServeCmd())
	return root
}

// Execute runs the root command and exits non-zero on failure.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

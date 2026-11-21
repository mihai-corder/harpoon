package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mihai-corder/harpoon/internal/report"
)

type ciFlags struct {
	env      string
	vars     []string
	only     []string
	jsonPath string
	htmlPath string
}

func newCiCmd() *cobra.Command {
	var f ciFlags
	cmd := &cobra.Command{
		Use:   "ci <collection.yaml>",
		Short: "Run a collection for CI: JSON/HTML reports + stable exit codes",
		Long: `Run a collection non-interactively for CI pipelines.

Execution semantics are identical to "run" (exit code 1 on any failure),
plus optional machine-readable reports:

  --json report.json   full run report: results, checks, failures, timings
  --html report.html   self-contained dark-themed HTML report page

The JSON shape is what the official GitHub Action in this repository
consumes to post PR comments.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCI(args[0], f)
		},
	}
	cmd.Flags().StringVarP(&f.env, "env", "e", "", "environment profile (harpoon.<env>.yaml next to the collection)")
	cmd.Flags().StringSliceVarP(&f.vars, "var", "V", nil, "override a variable, repeatable (KEY=VALUE)")
	cmd.Flags().StringSliceVarP(&f.only, "request", "r", nil, "run only named requests, comma-separated")
	cmd.Flags().StringVar(&f.jsonPath, "json", "", "write a JSON report to this path")
	cmd.Flags().StringVar(&f.htmlPath, "html", "", "write an HTML report to this path")
	return cmd
}

func runCI(path string, f ciFlags) error {
	started := time.Now()
	coll, results, err := loadAndRun(path, f.env, f.vars, f.only)
	if err != nil {
		return err
	}
	report.Print(os.Stdout, results)

	rep := report.BuildCIReport(coll.Name, started, results)

	// Reports are written even when requests fail, so CI still gets the
	// artifacts before the non-zero exit.
	if f.jsonPath != "" {
		if err := rep.WriteJSON(f.jsonPath); err != nil {
			return fmt.Errorf("write json report: %w", err)
		}
		fmt.Printf("json report: %s\n", f.jsonPath)
	}
	if f.htmlPath != "" {
		if err := rep.WriteHTML(f.htmlPath); err != nil {
			return fmt.Errorf("write html report: %w", err)
		}
		fmt.Printf("html report: %s\n", f.htmlPath)
	}

	finishOrExit(results)
	return nil
}

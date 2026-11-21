package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/report"
	"github.com/mihai-corder/harpoon/internal/runner"
)

type runFlags struct {
	env  string
	vars []string
	only []string
}

func newRunCmd() *cobra.Command {
	var f runFlags
	cmd := &cobra.Command{
		Use:   "run <collection.yaml>",
		Short: "Run a collection's requests and print a summary",
		Long: `Run a collection's requests in file order and print a summary table.

Variables are resolved in this order (highest wins):
CLI --var  >  extracted values  >  request-level vars  >  --env profile  >  collection vars.

Exits non-zero if any request fails, which makes the same collection
usable as a CI smoke test.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollection(args[0], f)
		},
	}
	cmd.Flags().StringVarP(&f.env, "env", "e", "", "environment profile (harpoon.<env>.yaml next to the collection)")
	cmd.Flags().StringSliceVarP(&f.vars, "var", "V", nil, "override a variable, repeatable (KEY=VALUE)")
	cmd.Flags().StringSliceVarP(&f.only, "request", "r", nil, "run only named requests, comma-separated")
	return cmd
}

func runCollection(path string, f runFlags) error {
	_, results, err := loadAndRun(path, f.env, f.vars, f.only)
	if err != nil {
		return err
	}
	report.Print(os.Stdout, results)
	finishOrExit(results)
	return nil
}

// loadAndRun loads the collection (plus profile and CLI overrides) and
// executes it. It is shared by the run and ci commands so both behave
// identically.
func loadAndRun(path, env string, varsRaw, only []string) (*config.Collection, []runner.Result, error) {
	coll, err := config.Load(path)
	if err != nil {
		return nil, nil, err
	}
	profile, err := config.LoadProfile(path, env)
	if err != nil {
		return nil, nil, err
	}
	cliVars, err := parseVars(varsRaw)
	if err != nil {
		return nil, nil, err
	}
	target := coll
	if len(only) > 0 {
		target, err = config.Select(coll, only)
		if err != nil {
			return nil, nil, err
		}
	}
	results := runner.Run(target, profile, runner.Options{CLIVars: cliVars})
	return target, results, nil
}

// finishOrExit prints the failure summary and exits non-zero, the contract
// CI systems rely on.
func finishOrExit(results []runner.Result) {
	passed := 0
	for _, r := range results {
		if r.Passed() {
			passed++
		}
	}
	if passed < len(results) {
		fmt.Fprintf(os.Stderr, "harpoon: %d of %d request(s) failed\n", len(results)-passed, len(results))
		os.Exit(1)
	}
}

func parseVars(pairs []string) (map[string]string, error) {
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --var %q (want KEY=VALUE)", p)
		}
		out[k] = v
	}
	return out, nil
}

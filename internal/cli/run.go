package cli

import (
	"fmt"
	"os"
	"sort"
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
	coll, err := config.Load(path)
	if err != nil {
		return err
	}
	profile, err := config.LoadProfile(path, f.env)
	if err != nil {
		return err
	}
	cliVars, err := parseVars(f.vars)
	if err != nil {
		return err
	}
	requests := coll.Requests
	if len(f.only) > 0 {
		requests, err = selectRequests(coll, f.only)
		if err != nil {
			return err
		}
	}

	target := &config.Collection{Name: coll.Name, Vars: coll.Vars, Requests: requests}
	results := runner.Run(target, profile, runner.Options{CLIVars: cliVars})
	report.Print(os.Stdout, results)

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
	return nil
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

func selectRequests(coll *config.Collection, names []string) ([]config.NamedRequest, error) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}

	out := make([]config.NamedRequest, 0, len(names))
	for _, nr := range coll.Requests {
		if want[nr.Name] {
			out = append(out, nr)
			delete(want, nr.Name)
		}
	}
	if len(want) > 0 {
		unknown := make([]string, 0, len(want))
		for n := range want {
			unknown = append(unknown, n)
		}
		sort.Strings(unknown)

		available := make([]string, 0, len(coll.Requests))
		for _, nr := range coll.Requests {
			available = append(available, nr.Name)
		}
		return nil, fmt.Errorf(
			"unknown request(s): %s (available: %s)",
			strings.Join(unknown, ", "), strings.Join(available, ", "))
	}
	return out, nil
}

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/interpolate"
)

func newValidateCmd() *cobra.Command {
	var env string
	cmd := &cobra.Command{
		Use:   "validate <collection.yaml>",
		Short: "Check a collection for structural and reference errors",
		Long: `Validate a collection file without sending any HTTP requests.

Structural errors (bad methods, missing URLs, duplicate names) fail the
command. Variables that are not defined in any static source are reported
as warnings, because they may legitimately be provided at runtime.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return validateCollection(args[0], env)
		},
	}
	cmd.Flags().StringVarP(&env, "env", "e", "", "environment profile for reference checking")
	return cmd
}

func validateCollection(path, env string) error {
	coll, err := config.Load(path)
	if err != nil {
		return err
	}
	profile, err := config.LoadProfile(path, env)
	if err != nil {
		return err
	}

	known := map[string]bool{}
	for k := range coll.Vars {
		known[k] = true
	}
	for k := range profile {
		known[k] = true
	}

	var warnings []string
	for _, nr := range coll.Requests {
		// Variables defined by this request become visible to it and every
		// later request, mirroring the runner's runtime scope.
		for k := range nr.Request.Vars {
			known[k] = true
		}
		for _, tpl := range templates(nr.Request) {
			for _, name := range interpolate.Placeholders(tpl) {
				if known[name] || strings.HasPrefix(name, "response.") {
					continue
				}
				warnings = append(warnings,
					fmt.Sprintf("warning: %s: request %q references undefined variable %q", path, nr.Name, name))
			}
		}
		// Values extracted by this request are known to later requests.
		for k := range nr.Request.Extract {
			known[k] = true
		}
	}

	fmt.Printf("✓ %s: %d request(s) valid\n", coll.Name, len(coll.Requests))
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	return nil
}

// templates returns every string that goes through interpolation for a
// request: URL, headers, vars, extract values and body strings.
func templates(r config.Request) []string {
	out := make([]string, 0, 4)
	out = append(out, r.URL)
	for _, v := range r.Headers {
		out = append(out, v)
	}
	for _, v := range r.Vars {
		out = append(out, v)
	}
	for _, v := range r.Extract {
		out = append(out, v)
	}
	_, _ = interpolate.WalkStrings(r.Body, func(s string) (string, error) {
		out = append(out, s)
		return s, nil
	})
	return out
}

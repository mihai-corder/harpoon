package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/tui"
)

func newTuiCmd() *cobra.Command {
	var env string
	var vars []string
	cmd := &cobra.Command{
		Use:   "tui <collection.yaml>",
		Short: "Open the interactive TUI for a collection",
		Long: `Open harpoon's interactive terminal UI.

Pick requests with ↑/↓, run them with enter (single) or R (all), and
inspect responses in the right pane. The session carries extracted
variables between runs, so login once and then re-run any authenticated
request on its own.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			coll, err := config.Load(args[0])
			if err != nil {
				return err
			}
			profile, err := config.LoadProfile(args[0], env)
			if err != nil {
				return err
			}
			cliVars, err := parseVars(vars)
			if err != nil {
				return err
			}

			program := tea.NewProgram(tui.New(coll, profile, cliVars), tea.WithAltScreen())
			_, err = program.Run()
			return err
		},
	}
	cmd.Flags().StringVarP(&env, "env", "e", "", "environment profile (harpoon.<env>.yaml next to the collection)")
	cmd.Flags().StringSliceVarP(&vars, "var", "V", nil, "override a variable, repeatable (KEY=VALUE)")
	return cmd
}

package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/mihai-corder/harpoon/internal/config"
	"github.com/mihai-corder/harpoon/internal/web"
)

func newServeCmd() *cobra.Command {
	var (
		env  string
		vars []string
		addr string
		open bool
	)
	cmd := &cobra.Command{
		Use:   "serve <collection.yaml>",
		Short: "Serve the web UI for a collection on localhost",
		Long: `Start a local server with harpoon's browser interface.

The UI is embedded in the binary — no frontend toolchain, no runtime
dependencies. The session carries extracted variables between runs, so
login once and then re-run any authenticated request on its own.`,
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

			handler := web.New(coll, profile, cliVars).Handler()
			url := fmt.Sprintf("http://localhost%s", addr)
			if open {
				go web.OpenBrowser(url)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			srv := &http.Server{Addr: addr, Handler: handler}
			errCh := make(chan error, 1)
			go func() { errCh <- srv.ListenAndServe() }()

			fmt.Printf("harpoon web UI: %s  (Ctrl+C to stop)\n", url)
			select {
			case err := <-errCh:
				if err != nil && err != http.ErrServerClosed {
					return err
				}
			case <-ctx.Done():
				_ = srv.Shutdown(context.Background())
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&env, "env", "e", "", "environment profile (harpoon.<env>.yaml next to the collection)")
	cmd.Flags().StringSliceVarP(&vars, "var", "V", nil, "override a variable, repeatable (KEY=VALUE)")
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	cmd.Flags().BoolVar(&open, "open", true, "open the browser automatically")
	return cmd
}

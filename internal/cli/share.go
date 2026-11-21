package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mihai-corder/harpoon/internal/share"
)

func newShareCmd() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "share <report.html>",
		Short: "Upload a report and get a public share link",
		Long: `Upload an HTML report (e.g. the one written by "harpoon ci --html") to a
harpoon-share server and print its public URL. Links expire after 90 days
and can be revoked with the delete token printed at upload time.

The default server is the maintainer-hosted instance; run your own with the
Worker in server/ and point at it via --server or $HARPOON_SHARE_SERVER.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return shareReport(args[0], server)
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "share server base URL (default: $HARPOON_SHARE_SERVER or the hosted instance)")
	return cmd
}

func shareReport(path, server string) error {
	if server == "" {
		server = os.Getenv("HARPOON_SHARE_SERVER")
	}
	if server == "" {
		server = share.DefaultServer
	}

	html, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fmt.Println("uploading report…")
	out, err := (&share.Client{Server: server}).Upload(html, "")
	if err != nil {
		return err
	}

	fmt.Printf("\n  shared:  %s\n", out.URL)
	fmt.Printf("  expires: %s\n", out.ExpiresAt)
	fmt.Printf("  revoke:  curl -X DELETE \"%s\" -H \"X-Delete-Token: %s\"\n", out.URL, out.DeleteToken)
	return nil
}

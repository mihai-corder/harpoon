package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mihai-corder/harpoon/internal/config"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <collection.yaml>",
		Short: "List the requests in a collection, in run order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			coll, err := config.Load(args[0])
			if err != nil {
				return err
			}

			nameW, methodW := 4, 6
			for _, nr := range coll.Requests {
				if l := len([]rune(nr.Name)); l > nameW {
					nameW = l
				}
				if l := len(nr.Request.Method); l > methodW {
					methodW = l
				}
			}
			fmt.Printf("  %-*s  %-*s  %s\n", nameW, "NAME", methodW, "METHOD", "URL")
			for _, nr := range coll.Requests {
				fmt.Printf("  %-*s  %-*s  %s\n", nameW, nr.Name, methodW, nr.Request.Method, nr.Request.URL)
			}
			return nil
		},
	}
}

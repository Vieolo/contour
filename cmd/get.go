package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/store"
)

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Print a single item's content",
	Long: "Print the body of one item, identified by the ID shown in " +
		"`contour list` — for example: contour get rules/go/errors\n\n" +
		"Only the body is written to stdout, so it can be piped or captured " +
		"directly by an agent.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := resolveStore()
		if err != nil {
			return err
		}
		st, err := store.Load(home.Path)
		if err != nil {
			return err
		}

		id := strings.TrimSuffix(args[0], "/")
		it, ok := st.Get(id)
		if !ok {
			return fmt.Errorf("no item with ID %q (run `%s list` to see the available IDs)", args[0], config.Program)
		}

		fmt.Println(it.Body)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/contour/internal/unic"
)

type getInput struct {
	ID string `json:"id" jsonschema:"the item's ID, for example rules/python/010-errors"`
}

var getCmd = unic.UniversalCommand[getInput, any]{
	Use:   "get <id>",
	Short: "Print a single item's content",
	Long: "Print the body of one item, identified by the ID shown in " +
		"`contour list` — for example: contour get rules/python/errors\n\n" +
		"Only the body is written to stdout, so it can be piped or captured " +
		"directly by an agent.",
	Description: "Read the full content of a single contour item by its ID, " +
		"as listed by the list or search tools.",
	Args: cobra.ExactArgs(1),

	CLICommand: func(cmd *cobra.Command, args []string) error {
		it, found, err := lookupItem(args[0])
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("no item with ID %q (run `%s list` to see the available IDs)", args[0], config.Program)
		}

		fmt.Println(it.Body)
		return nil
	},

	MCPCommand: func(ctx context.Context, req *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
		it, found, err := lookupItem(in.ID)
		if err != nil {
			return nil, nil, asToolError(err)
		}
		if !found {
			return nil, nil, mcpserver.NotFound(
				fmt.Sprintf("no item with ID %q", in.ID),
				"call the list tool to see the valid IDs")
		}
		return mcpserver.TextResult(it.Body, "(this item has no content)"), nil, nil
	},
}

func init() {
	addCommand(&getCmd)
}

// lookupItem resolves the store and fetches one item by ID, reporting whether
// it was found separately from real failures.
//
// Both surfaces share the lookup but not the not-found wording: the CLI points
// at `contour list`, while the tool points at the list tool. Telling a model to
// run a shell command would be actively misleading.
func lookupItem(id string) (item store.Item, found bool, err error) {
	home, err := resolveStore()
	if err != nil {
		return store.Item{}, false, err
	}
	st, err := store.Load(home.Path)
	if err != nil {
		return store.Item{}, false, err
	}

	it, ok := st.Get(strings.TrimSuffix(strings.TrimSpace(id), "/"))
	return it, ok, nil
}

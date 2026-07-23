package cmd

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/contour/internal/unic"
)

type listInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"restrict to a single kind: rules, skills or knowledge"`
}

var listCmd = unic.UniversalCommand[listInput, any]{
	Use:   "list [kind]",
	Short: "List the items in the contour store",
	Long: "List the store's items with their tags and descriptions. Optionally " +
		"restrict to a single kind: rules, skills or knowledge.",
	Description: "List the items available in the contour store with their IDs, descriptions and tags. " +
		"Optionally restrict to a single kind. Pass a returned ID to the get tool to read its content.",
	Args: cobra.MaximumNArgs(1),

	CLICommand: func(cmd *cobra.Command, args []string) error {
		home, err := resolveStore()
		if err != nil {
			return err
		}

		kind := ""
		if len(args) == 1 {
			kind = args[0]
		}
		st, kinds, err := store.LoadForKind(home.Path, kind)
		if err != nil {
			return err
		}

		printStoreHeader(home.Path)
		for _, k := range kinds {
			printKindSection(k, st.ByKind(k))
		}
		return nil
	},

	MCPCommand: func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		home, err := resolveStore()
		if err != nil {
			return nil, nil, err
		}
		st, kinds, err := store.LoadForKind(home.Path, in.Kind)
		if err != nil {
			return nil, nil, err
		}

		var b strings.Builder
		for _, k := range kinds {
			b.WriteString(bootstrap.RenderMenu(string(k), st.ByKind(k), ""))
		}
		return mcpserver.TextResult(b.String(), "The store contains no items."), nil, nil
	},
}

func init() {
	addCommand(&listCmd)
}

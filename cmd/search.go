package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/contour/internal/unic"
	"github.com/vieolo/termange"
)

type searchInput struct {
	Query string `json:"query" jsonschema:"text matched against item IDs, descriptions, tags and content"`
	Kind  string `json:"kind,omitempty" jsonschema:"restrict to a single kind: rules, skills or knowledge"`
}

var searchCmd = unic.UniversalCommand[searchInput, any]{
	Use:   "search <query> [kind]",
	Short: "Search the store for items matching a query",
	Long: "Search the store for items whose ID, description, tags or content " +
		"match a query, case-insensitively. Optionally restrict to a single " +
		"kind: rules, skills or knowledge.",
	Description: "Search the contour store for items whose ID, description, tags or content match a query. " +
		"Returns a menu of matches; use the get tool to read one.",
	Args: cobra.RangeArgs(1, 2),

	CLICommand: func(cmd *cobra.Command, args []string) error {
		home, err := resolveStore()
		if err != nil {
			return err
		}

		kind := ""
		if len(args) == 2 {
			kind = args[1]
		}
		st, kinds, err := store.LoadForKind(home.Path, kind)
		if err != nil {
			return err
		}

		printStoreHeader(home.Path)
		found := 0
		for _, k := range kinds {
			hits := st.Search(k, args[0])
			found += len(hits)
			printKindSection(k, hits)
		}
		if found == 0 {
			fmt.Println()
			termange.PrintWarningf("No items match %q.\n", args[0])
		}
		return nil
	},

	MCPCommand: func(ctx context.Context, req *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
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
			b.WriteString(bootstrap.RenderMenu(string(k), st.Search(k, in.Query), ""))
		}
		return mcpserver.TextResult(b.String(), fmt.Sprintf("No items match %q.", in.Query)), nil, nil
	},
}

func init() {
	addCommand(&searchCmd)
}

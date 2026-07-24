package cmd

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/render"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/contour/internal/unic"
)

type listInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"restrict to a single kind: rules, skills or knowledge"`
}

// listResult is the structured output of the list tool. The get tool's body is
// prose and stays text-only, but a listing is data, so it carries a schema an
// agent can read without parsing the menu.
type listResult struct {
	Items []listItem `json:"items"`
}

type listItem struct {
	ID          string   `json:"id" jsonschema:"pass to the get tool to read the full item"`
	Kind        string   `json:"kind"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

var listCmd = unic.UniversalCommand[listInput, listResult]{
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

		render.StoreHeader(home.Path)
		for _, k := range kinds {
			render.KindSection(k, st.ByKind(k))
		}
		return nil
	},

	MCPCommand: func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, listResult, error) {
		home, err := resolveStore()
		if err != nil {
			return nil, listResult{}, asToolError(err)
		}
		st, kinds, err := store.LoadForKind(home.Path, in.Kind)
		if err != nil {
			return nil, listResult{}, asToolError(err)
		}

		result := listResult{Items: []listItem{}}
		var b strings.Builder
		for _, k := range kinds {
			items := st.ByKind(k)
			b.WriteString(bootstrap.RenderMenu(string(k), items, ""))
			for _, it := range items {
				result.Items = append(result.Items, listItem{
					ID:          it.ID,
					Kind:        string(it.Kind),
					Description: it.Description,
					Tags:        it.Tags,
				})
			}
		}
		return mcpserver.TextResult(b.String(), "The store contains no items."), result, nil
	},
}

func init() {
	addCommand(&listCmd)
}

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/render"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/contour/internal/unic"
)

// Search emits a bounded sample of matching lines. A real store can turn up
// dozens of files with many matches each; reporting every line would make the
// result heavier than just fetching the top items, which is the opposite of
// what search is for. So the counts are always exact and the lines are sampled.
const (
	searchMaxLines   = 3
	searchMaxLineLen = 200
)

type searchInput struct {
	Query string `json:"query" jsonschema:"text matched against item IDs, descriptions, tags and content"`
	Kind  string `json:"kind,omitempty" jsonschema:"restrict to a single kind: rules, skills or knowledge"`
}

// searchResult is the structured output of the search tool. Its schema is what
// lets an agent read the outcome — file counts, match counts, line locations —
// without parsing prose.
type searchResult struct {
	Query        string      `json:"query" jsonschema:"the query that was searched for"`
	FileCount    int         `json:"file_count" jsonschema:"number of items that matched"`
	TotalMatches int         `json:"total_matches" jsonschema:"total occurrences of the query across all item bodies"`
	Results      []searchHit `json:"results"`
}

type searchHit struct {
	ID          string       `json:"id" jsonschema:"pass to the get tool to read the full item"`
	Kind        string       `json:"kind"`
	Description string       `json:"description,omitempty"`
	MatchedIn   []string     `json:"matched_in" jsonschema:"where the query matched: any of id, description, tags, body"`
	MatchCount  int          `json:"match_count" jsonschema:"occurrences of the query in this item's body"`
	Lines       []searchLine `json:"lines,omitempty" jsonschema:"a bounded sample of matching body lines"`
	Truncated   bool         `json:"truncated" jsonschema:"true when more matching body lines exist than are listed"`
}

type searchLine struct {
	Line int    `json:"line" jsonschema:"1-based line number within the item body as the get tool returns it"`
	Text string `json:"text"`
}

var searchCmd = unic.UniversalCommand[searchInput, searchResult]{
	Use:   "search <query> [kind]",
	Short: "Search the store for items matching a query",
	Long: "Search the store for items whose ID, description, tags or content " +
		"match a query, case-insensitively. Optionally restrict to a single " +
		"kind: rules, skills or knowledge.",
	Description: "Search the contour store for items whose ID, description, tags or content match a query. " +
		"Returns, per matching item, where it matched and a sample of the matching lines with counts; " +
		"pass an ID to the get tool to read the whole item.",
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
		st, err := loadProjectStore(home.Path)
		if err != nil {
			return err
		}
		kinds, err := resolveKinds(kind)
		if err != nil {
			return err
		}

		render.StoreHeader(home.Path)
		occurrences, files := 0, 0
		for _, k := range kinds {
			hits := st.Search(k, args[0])
			render.SearchKindHeader(k, len(hits))
			for _, m := range hits {
				render.SearchHit(m, searchMaxLines, searchMaxLineLen)
				occurrences += m.Occurrences
				files++
			}
		}
		if files == 0 {
			render.NoMatches(args[0])
			return nil
		}
		render.SearchSummary(occurrences, files, args[0])
		return nil
	},

	MCPCommand: func(ctx context.Context, req *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchResult, error) {
		home, err := resolveStore()
		if err != nil {
			return nil, searchResult{}, asToolError(err)
		}
		st, err := loadProjectStore(home.Path)
		if err != nil {
			return nil, searchResult{}, asToolError(err)
		}
		kinds, err := resolveKinds(in.Kind)
		if err != nil {
			return nil, searchResult{}, asToolError(err)
		}

		result := buildSearchResult(st, kinds, in.Query)
		// FileCount is the gap signal: zero means the agent looked and found
		// nothing.
		mcpUsage.Search(in.Query, in.Kind, result.FileCount)

		// The structured result is the contract; the text is a readable
		// rendering of the same data, so the outcome reaches the model whether
		// the client forwards structured content or only text.
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: renderSearchText(result)}},
		}
		return res, result, nil
	},
}

func init() {
	addCommand(&searchCmd)
}

func buildSearchResult(st *store.Store, kinds []store.Kind, query string) searchResult {
	result := searchResult{Query: query, Results: []searchHit{}}

	for _, k := range kinds {
		for _, m := range st.Search(k, query) {
			hit := searchHit{
				ID:          m.Item.ID,
				Kind:        string(m.Item.Kind),
				Description: m.Item.Description,
				MatchedIn:   m.MatchedIn(),
				MatchCount:  m.Occurrences,
				Truncated:   len(m.Lines) > searchMaxLines,
			}
			for i, ln := range m.Lines {
				if i >= searchMaxLines {
					break
				}
				hit.Lines = append(hit.Lines, searchLine{
					Line: ln.Number,
					Text: render.TruncateLine(ln.Text, searchMaxLineLen),
				})
			}
			result.Results = append(result.Results, hit)
			result.TotalMatches += m.Occurrences
		}
	}

	result.FileCount = len(result.Results)
	return result
}

// renderSearchText mirrors the structured result as plain text for the tool's
// content block.
func renderSearchText(r searchResult) string {
	if len(r.Results) == 0 {
		return fmt.Sprintf("No items match %q.", r.Query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d match(es) for %q across %d item(s).\n", r.TotalMatches, r.Query, r.FileCount)
	for _, hit := range r.Results {
		b.WriteString("\n")
		if hit.Description != "" {
			fmt.Fprintf(&b, "%s — %s\n", hit.ID, hit.Description)
		} else {
			fmt.Fprintf(&b, "%s\n", hit.ID)
		}

		if len(hit.Lines) == 0 {
			fmt.Fprintf(&b, "  matched in: %s\n", strings.Join(hit.MatchedIn, ", "))
			continue
		}
		fmt.Fprintf(&b, "  %d match(es) in body:\n", hit.MatchCount)
		for _, ln := range hit.Lines {
			fmt.Fprintf(&b, "  %d: %s\n", ln.Line, ln.Text)
		}
		if hit.Truncated {
			b.WriteString("  … more matching lines; use get to read the whole item\n")
		}
	}
	return b.String()
}

package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/store"
)

type listInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"restrict to a single kind: rules, skills or knowledge"`
}

type getInput struct {
	ID string `json:"id" jsonschema:"the item's ID, for example rules/go/010-errors"`
}

type searchInput struct {
	Query string `json:"query" jsonschema:"text matched against item IDs, descriptions, tags and content"`
	Kind  string `json:"kind,omitempty" jsonschema:"restrict to a single kind: rules, skills or knowledge"`
}

// registerTools installs the tools that make the store reachable on demand.
//
// Each handler reloads the store, so edits to the files on disk take effect
// without restarting the server. Stores are small, and always-fresh reads beat
// a stale cache.
func registerTools(s *mcp.Server, root string) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "list",
		Description: "List the items available in the contour store with their IDs, descriptions and tags. " +
			"Optionally restrict to a single kind. Pass a returned ID to the get tool to read its content.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		st, kinds, err := loadForKind(root, in.Kind)
		if err != nil {
			return nil, nil, err
		}

		var b strings.Builder
		for _, k := range kinds {
			b.WriteString(bootstrap.RenderMenu(string(k), st.ByKind(k), ""))
		}
		return textResult(b.String(), "The store contains no items."), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get",
		Description: "Read the full content of a single contour item by its ID, " +
			"as listed by the list or search tools.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
		st, err := store.Load(root)
		if err != nil {
			return nil, nil, err
		}

		it, ok := st.Get(strings.TrimSpace(in.ID))
		if !ok {
			return nil, nil, fmt.Errorf("no item with ID %q; use the list tool to see the valid IDs", in.ID)
		}
		return textResult(it.Body, "(this item has no content)"), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "search",
		Description: "Search the contour store for items whose ID, description, tags or content match a query. " +
			"Returns a menu of matches; use the get tool to read one.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
		st, kinds, err := loadForKind(root, in.Kind)
		if err != nil {
			return nil, nil, err
		}

		var b strings.Builder
		for _, k := range kinds {
			var hits []store.Item
			for _, it := range st.ByKind(k) {
				if matches(it, in.Query) {
					hits = append(hits, it)
				}
			}
			b.WriteString(bootstrap.RenderMenu(string(k), hits, ""))
		}
		return textResult(b.String(), fmt.Sprintf("No items match %q.", in.Query)), nil, nil
	})
}

// loadForKind loads the store and resolves an optional kind filter; an empty
// kind means every kind.
func loadForKind(root, kind string) (*store.Store, []store.Kind, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(kind) == "" {
		return st, store.Kinds, nil
	}

	k, err := store.ParseKind(kind)
	if err != nil {
		return nil, nil, err
	}
	return st, []store.Kind{k}, nil
}

// matches reports whether an item satisfies a case-insensitive query across its
// ID, description, tags and body.
func matches(it store.Item, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(it.ID), q) ||
		strings.Contains(strings.ToLower(it.Description), q) ||
		strings.Contains(strings.ToLower(it.Body), q) {
		return true
	}
	for _, t := range it.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

// textResult wraps text as tool output, substituting fallback when it is empty
// so the model never receives a blank response.
func textResult(text, fallback string) *mcp.CallToolResult {
	text = strings.TrimSpace(text)
	if text == "" {
		text = fallback
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

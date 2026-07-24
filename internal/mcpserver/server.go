// Package mcpserver builds and runs the MCP server that exposes a contour store
// to a client over stdio.
//
// It owns the server's lifetime and its initialisation instructions — the eager
// bootstrap payload — but not the tools. Those are defined alongside their CLI
// counterparts under cmd/ and attached by the caller, so a command's two
// surfaces live in one file.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/store"
)

// fetchHint tells an agent how to retrieve a menu item over MCP. The CLI passes
// its own hint to the same renderer.
const fetchHint = "the `get` tool with the item's ID"

// Options configures the server.
type Options struct {
	// Root is the contour store's directory.
	Root string

	// Profile is the bootstrap profile loaded eagerly. When empty no rules are
	// preloaded, and the instructions explain how to select one.
	Profile string

	// Version is reported to clients during initialisation.
	Version string
}

// New builds the server with the bootstrap payload as its initialisation
// instructions. The caller registers tools before serving.
func New(opts Options) (*mcp.Server, error) {
	instructions, err := BuildInstructions(opts)
	if err != nil {
		return nil, err
	}

	return mcp.NewServer(&mcp.Implementation{
		Name:    "contour",
		Title:   "contour context provider",
		Version: opts.Version,
	}, &mcp.ServerOptions{
		Instructions: instructions,
	}), nil
}

// Serve runs the server on stdio, blocking until the client disconnects or ctx
// is cancelled.
//
// A client closing the pipe, or a signal cancelling ctx, is an ordinary
// shutdown rather than a failure worth reporting to the user.
//
// Nothing may be written to stdout but protocol traffic; callers must send
// their diagnostics to stderr.
func Serve(ctx context.Context, s *mcp.Server) error {
	if err := s.Run(ctx, &mcp.StdioTransport{}); err != nil &&
		!errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// TextResult wraps text as tool output, substituting fallback when it is empty
// so the model never receives a blank response.
func TextResult(text, fallback string) *mcp.CallToolResult {
	text = strings.TrimSpace(text)
	if text == "" {
		text = fallback
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// BuildInstructions composes what the client receives at initialisation: a short
// preamble, the profile's eager rules, and menus of what can be fetched.
func BuildInstructions(opts Options) (string, error) {
	st, err := store.Load(opts.Root)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("contour provides the centralised rules, skills and knowledge for this session.\n")
	b.WriteString("Any rules below are already in effect. Use the `list`, `search` and `get` tools to pull anything else on demand.\n\n")

	if opts.Profile == "" {
		writeNoProfileNotice(&b, opts.Root)
		for _, k := range store.Kinds {
			b.WriteString(bootstrap.RenderMenu("Available "+string(k), st.ByKind(k), fetchHint))
		}
		return strings.TrimRight(b.String(), "\n") + "\n", nil
	}

	p, err := bootstrap.LoadProfile(opts.Root, opts.Profile)
	if err != nil {
		return "", err
	}
	b.WriteString(bootstrap.Compose(p, st).Render(fetchHint))
	return b.String(), nil
}

// writeNoProfileNotice explains how to select an entry point when the server was
// started without one, naming the profiles that exist.
func writeNoProfileNotice(b *strings.Builder, root string) {
	b.WriteString("No bootstrap profile is selected, so no rules were loaded eagerly.\n")
	b.WriteString("Select one by passing --bootstrap <name> when starting the server.\n")

	profiles, err := bootstrap.LoadProfiles(root)
	if err != nil || len(profiles) == 0 {
		b.WriteString("\n")
		return
	}

	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	fmt.Fprintf(b, "Available profiles: %s\n\n", strings.Join(names, ", "))
}

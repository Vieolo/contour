// Package mcpserver exposes a contour store to an MCP client over stdio.
//
// It mirrors the CLI's progressive disclosure. The selected bootstrap profile's
// rules are delivered eagerly in the server's initialisation instructions, while
// skills and knowledge stay one tool call away via list, search and get. That
// keeps a session's starting context small without putting the store out of
// reach.
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

// Run serves the store over stdio, blocking until the client disconnects or ctx
// is cancelled.
//
// Nothing may be written to stdout but protocol traffic; callers must send
// their diagnostics to stderr.
func Run(ctx context.Context, opts Options) error {
	instructions, err := buildInstructions(opts)
	if err != nil {
		return err
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "contour",
		Title:   "contour context provider",
		Version: opts.Version,
	}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	registerTools(server, opts.Root)

	// A client closing the pipe, or a signal cancelling ctx, is an ordinary
	// shutdown rather than a failure worth reporting to the user.
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil &&
		!errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// buildInstructions composes what the client receives at initialisation: a short
// preamble, the profile's eager rules, and menus of what can be fetched.
func buildInstructions(opts Options) (string, error) {
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
	b.WriteString("Select one with the --bootstrap flag or the CONTOUR_BOOTSTRAP environment variable.\n")

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

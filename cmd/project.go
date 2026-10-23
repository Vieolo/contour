package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/project"
	"github.com/vieolo/contour/internal/store"
)

// mcpPreset names the preset the running MCP server started with. It is empty
// outside `contour mcp`, where commands act as the default preset.
var mcpPreset string

// projectContext gathers what the working directory contributes to a load: the
// overlay directories and the project config. It uses the working directory for
// the same reason usage logging does — an MCP client spawns the server in the
// project, and a person runs the CLI from it.
//
// A config that cannot be read is reported and treated as absent rather than
// failing the command: an MCP server that refused to start would take the
// project's local rules down with it.
func projectContext() (overlays []string, cfg project.Config) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, project.Config{}
	}
	overlays = store.DiscoverOverlays(wd)

	cfg, err = project.Load(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "contour: ignoring the project config: %v\n", err)
		return overlays, project.Config{}
	}
	// Config that was found but not used, or just migrated, is worth saying out
	// loud. Diagnostics go to stderr, leaving stdout clean for a piped payload
	// and pure for the MCP protocol.
	for _, w := range cfg.Warnings {
		fmt.Fprintf(os.Stderr, "contour: %s\n", w)
	}
	return overlays, cfg
}

// loadProjectStore loads what the list, search and get commands reach: the
// central store merged with the working directory's overlays and the eager
// files of every preset.
//
// Every preset's, not just the session's: a preset decides what is offered to
// an agent up front, never what it can reach. An agent in a frontend session
// can still search the backend's AGENTS.md.
func loadProjectStore(central string) (*store.Store, error) {
	overlays, cfg := projectContext()
	var eager []store.EagerFile
	for _, p := range cfg.Presets {
		eager = append(eager, p.EagerFiles...)
	}
	return store.LoadProject(central, overlays, eager)
}

// loadPresetStore loads the store as one preset sees it: the central store and
// overlays, plus that preset's eager files alone. Composing a payload needs
// exactly this — the eager files in the store are always delivered, so another
// preset's here would leak into this one's.
func loadPresetStore(central string, overlays []string, preset project.Preset) (*store.Store, error) {
	return store.LoadProject(central, overlays, preset.EagerFiles)
}

// profileSources looks profiles up in the project's overlays, then the central
// store.
func profileSources(central string, overlays []string) bootstrap.Sources {
	return bootstrap.Sources{Central: central, Overlays: overlays}
}

// resolveKinds turns an optional kind argument into the kinds to show.
func resolveKinds(kind string) ([]store.Kind, error) {
	if strings.TrimSpace(kind) == "" {
		return store.Kinds, nil
	}
	k, err := store.ParseKind(kind)
	if err != nil {
		return nil, err
	}
	return []store.Kind{k}, nil
}

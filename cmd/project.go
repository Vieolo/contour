package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/vieolo/contour/internal/project"
	"github.com/vieolo/contour/internal/store"
)

// projectContext gathers what the working directory contributes to a load: the
// overlay directories, and the eager files and bootstrap profiles from the
// project config. It uses the working directory for the same reason usage
// logging does — an MCP client spawns the server in the project, and a person
// runs the CLI from it.
func projectContext() (overlays []string, eagerFiles []store.EagerFile, bootstrap []string) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, nil, nil
	}
	overlays = store.DiscoverOverlays(wd)
	if cfg, err := project.Load(wd); err == nil {
		eagerFiles = cfg.EagerFiles
		bootstrap = cfg.Bootstrap
		// Config that was found but not used is worth saying out loud — silently
		// ignoring a second config file is how a project ends up mystified about
		// which settings are in force. Diagnostics go to stderr, leaving stdout
		// clean for a piped payload and pure for the MCP protocol.
		for _, w := range cfg.Warnings {
			fmt.Fprintf(os.Stderr, "contour: %s\n", w)
		}
	}
	return overlays, eagerFiles, bootstrap
}

// loadProjectStore loads the central store merged with the working directory's
// overlays and eager files.
func loadProjectStore(central string) (*store.Store, error) {
	overlays, eagerFiles, _ := projectContext()
	return store.LoadProject(central, overlays, eagerFiles)
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

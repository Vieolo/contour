package cmd

import (
	"os"

	"github.com/vieolo/contour/internal/store"
)

// projectOverlays returns the recognised overlay directories in the working
// directory the command was invoked in — the project the store is layered over.
//
// It uses the working directory for the same reason usage logging does: an MCP
// client spawns the server in the project, and a person runs the CLI from it.
func projectOverlays() []string {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	return store.DiscoverOverlays(wd)
}

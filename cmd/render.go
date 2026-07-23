package cmd

import (
	"fmt"
	"strings"

	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/termange"
)

// This file holds the CLI's human-facing rendering, shared by the commands that
// list items. The MCP surfaces of those same commands render through
// bootstrap.RenderMenu instead: a terminal wants colour and indentation, while a
// model wants plain, compact text.

// printStoreHeader announces which store the output came from, with a marker in
// development builds so dev and production output are never confused.
func printStoreHeader(root string) {
	if config.Dev {
		termange.PrintWarningln(config.Label + " build — using the dev store")
	}
	termange.PrintInfof("contour store: %s\n", root)
}

// printKindSection prints one kind's items, or a placeholder when it has none.
func printKindSection(kind store.Kind, items []store.Item) {
	fmt.Println()
	termange.PrintColorf(termange.ColorGreen, "%s (%d)\n", string(kind), len(items))
	if len(items) == 0 {
		termange.PrintInfoln("  (none)")
		return
	}
	for _, it := range items {
		printItem(it)
	}
}

// printItem renders one item for a human: ID, description and tags.
func printItem(it store.Item) {
	termange.PrintInfof("  %s\n", it.ID)
	if it.Description != "" {
		termange.PrintInfof("      %s\n", it.Description)
	}
	if len(it.Tags) > 0 {
		termange.PrintColorf(termange.ColorYellow, "      tags: %s\n", strings.Join(it.Tags, ", "))
	}
}

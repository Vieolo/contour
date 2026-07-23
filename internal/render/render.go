// Package render draws contour's terminal output: colourised, indented and
// meant to be read by a person.
//
// It is the human counterpart to bootstrap.RenderMenu, which renders the same
// items as plain, compact text for a model's context. Commands that expose both
// surfaces call one from each handler, which is why the two live apart.
package render

import (
	"fmt"
	"strings"

	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/termange"
)

// StoreHeader announces which store the output came from, with a marker in
// development builds so dev and production output are never confused.
func StoreHeader(root string) {
	if config.Dev {
		termange.PrintWarningln(config.Label + " build — using the dev store")
	}
	termange.PrintInfof("contour store: %s\n", root)
}

// KindSection prints one kind's items, or a placeholder when it has none.
func KindSection(kind store.Kind, items []store.Item) {
	fmt.Println()
	termange.PrintColorf(termange.ColorGreen, "%s (%d)\n", string(kind), len(items))
	if len(items) == 0 {
		termange.PrintInfoln("  (none)")
		return
	}
	for _, it := range items {
		item(it)
	}
}

// NoMatches reports an empty search result.
func NoMatches(query string) {
	fmt.Println()
	termange.PrintWarningf("No items match %q.\n", query)
}

// item renders a single item: ID, description and tags.
func item(it store.Item) {
	termange.PrintInfof("  %s\n", it.ID)
	if it.Description != "" {
		termange.PrintInfof("      %s\n", it.Description)
	}
	if len(it.Tags) > 0 {
		termange.PrintColorf(termange.ColorYellow, "      tags: %s\n", strings.Join(it.Tags, ", "))
	}
}

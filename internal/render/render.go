// Package render draws contour's terminal output: colourised, indented and
// meant to be read by a person.
//
// It is the human counterpart to bootstrap.RenderMenu, which renders the same
// items as plain, compact text for a model's context. Commands that expose both
// surfaces call one from each handler, which is why the two live apart.
package render

import (
	"fmt"
	"os"
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

// StoreCreated reports that contour set up the store, and explains how its
// layout works. First run is the one moment the user is guaranteed to see this,
// so it teaches the structure rather than just naming the path.
//
// Unlike the rest of this package it writes to stderr: commands such as get and
// bootstrap emit a payload on stdout that a banner would corrupt.
func StoreCreated(path string) {
	rule := termange.PaintText(strings.Repeat("─", 64), termange.ColorYellow)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n\n", rule,
		termange.PaintText("  contour — store created", termange.ColorYellow), rule)
	fmt.Fprintf(&b, "No store existed yet, so contour created one for you at:\n\n    %s\n\n",
		termange.PaintText(path, termange.ColorGreen))
	b.WriteString(storeLayoutHelp)
	fmt.Fprintf(&b, "\n%s\n    %s set-home /path/to/store\n\n",
		termange.PaintText("  To keep the store somewhere else", termange.ColorGreen), config.Program)
	fmt.Fprintf(&b, "%s\n", rule)

	fmt.Fprint(os.Stderr, b.String())
}

const storeLayoutHelp = `Layout:

    bootstrap/   Named entry points. Each profile selects, by tag, which
                 rules load eagerly and which skills and knowledge are
                 offered on demand.
    rules/       Imperative "how to behave" guidance. Loaded eagerly.
    skills/      Procedural "how to do X". Fetched on demand.
    knowledge/   Reference facts. Fetched on demand.

Folder names under rules/, skills/ and knowledge/ become implicit tags, so a
file at rules/python/errors.md is tagged "python". Sample files are included;
edit or delete them freely, and see README.md in the store for the conventions.
`

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

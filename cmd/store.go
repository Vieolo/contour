package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/termange"
)

// resolveStore locates the contour store for commands that read from it.
//
// The store is a single central directory set up once, not a per-project
// scaffold, so a missing default location is not a failure: contour creates the
// store, explains its layout, and carries on. An explicitly configured location
// that does not exist is a different matter — that is a misconfiguration, and
// silently creating a store somewhere the user did not expect would be worse
// than saying so.
//
// Notices go to stderr, keeping stdout free for output that may be piped into
// an agent.
func resolveStore() (config.Home, error) {
	home, err := config.Resolve()
	if err != nil {
		return config.Home{}, err
	}
	if home.Exists {
		return home, nil
	}

	if home.Explicit {
		return config.Home{}, fmt.Errorf(
			"%s points to %q, but that directory does not exist; create it, point %s at your store, or unset it to use the default",
			config.EnvVar, home.Path, config.EnvVar)
	}

	if err := scaffoldStore(home.Path); err != nil {
		return config.Home{}, err
	}
	printStoreCreated(home.Path)

	home.Exists = true
	return home, nil
}

// printStoreCreated reports, on stderr, that contour set up the store and how
// its layout works. First run is the one moment the user is guaranteed to see
// this, so it teaches the structure rather than just naming the path.
func printStoreCreated(path string) {
	rule := termange.PaintText(strings.Repeat("─", 64), termange.ColorYellow)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n\n", rule,
		termange.PaintText("  contour — store created", termange.ColorYellow), rule)
	fmt.Fprintf(&b, "No store existed yet, so contour created one for you at:\n\n    %s\n\n",
		termange.PaintText(path, termange.ColorGreen))
	b.WriteString(storeLayoutHelp)
	fmt.Fprintf(&b, "\n%s\n    move the directory, then set %s to its new path\n\n",
		termange.PaintText("  To keep the store somewhere else", termange.ColorGreen), config.EnvVar)
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
file at rules/go/errors.md is tagged "go". Sample files are included — edit
or delete them freely; README.md in the store explains the conventions.
`

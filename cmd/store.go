package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/termange"
)

// resolveStore locates the contour store for commands that read from it. When
// the store is missing it prints the appropriate guidance and terminates the
// process: a hard error for an explicitly configured path, or the onboarding
// banner for a missing default. It only returns on success.
func resolveStore() config.Home {
	home, err := config.Resolve()
	if err != nil {
		termange.PrintErrorln(err.Error())
		os.Exit(1)
	}

	if !home.Exists {
		if home.Explicit {
			termange.PrintErrorf("%s points to %q, but that directory does not exist.\n", config.EnvVar, home.Path)
			termange.PrintErrorf("Create it, point %s at your store, or run `%s init`.\n", config.EnvVar, config.Program)
		} else {
			printNoStoreBanner(home.Path)
		}
		os.Exit(1)
	}

	return home
}

// printNoStoreBanner explains, to a human or an agent reading stdout, that no
// store exists yet at the default location and how to create or relocate it.
func printNoStoreBanner(defaultPath string) {
	rule := strings.Repeat("─", 64)

	termange.PrintColorln(rule, termange.ColorYellow)
	termange.PrintColorln("  contour — no store found", termange.ColorYellow)
	termange.PrintColorln(rule, termange.ColorYellow)
	fmt.Println()
	termange.PrintInfoln("contour looked for your context store at:")
	fmt.Println()
	termange.PrintColorln("    "+defaultPath, termange.ColorWhite)
	fmt.Println()
	termange.PrintInfoln("but that directory does not exist yet. The store holds the rules,")
	termange.PrintInfoln("skills and knowledge that contour feeds to your AI agents.")
	fmt.Println()
	termange.PrintColorln("  Create it here", termange.ColorGreen)
	termange.PrintInfof("      %s init\n", config.Program)
	fmt.Println()
	termange.PrintColorln("  Put it somewhere else", termange.ColorGreen)
	termange.PrintInfof("      set %s=/path/to/your/store, then run: %s init\n", config.EnvVar, config.Program)
	fmt.Println()
	termange.PrintColorln(rule, termange.ColorYellow)
}

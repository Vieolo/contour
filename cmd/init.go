package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/scaffold"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/termange"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the contour store skeleton",
	Long: "Create the contour store directory and its folder structure " +
		"(bootstrap, rules, skills, knowledge), a README describing the " +
		"convention, and a few sample files.\n\n" +
		"Running this is optional: contour creates the store on first use when " +
		"it is missing. The command exists for when you want to set the store " +
		"up explicitly — notably at a store_path you have set in the config " +
		"file, which contour will not create on its own.\n\n" +
		"init only ever creates a store. If one already exists — any directory " +
		"with content in it — it is left exactly as it is: no sample you deleted " +
		"comes back, and nothing is overwritten.\n\n" +
		"With --local, it creates a store for the project in the working " +
		"directory instead: a " + store.DefaultOverlayDir + "/ folder holding " +
		"rules/, skills/ and knowledge/, which contour merges over the central " +
		"store whenever it runs in this project. Everything put there is live " +
		"at once — local rules load into every session, no profile needed — so " +
		"it gets no sample files, only a README documenting the formats. A " +
		"project that already has a local store in any overlay folder (.contour/, " +
		".agents/ or .claude/) is left as it is. --local touches neither the " +
		"central store nor its config.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if initLocal {
			return runInitLocal()
		}

		// Resolve rather than resolveStore: init is the command that creates
		// the store, so a missing directory is the normal case here — including
		// at an explicitly configured path, which resolveStore rejects.
		home, err := config.Resolve()
		if err != nil {
			return err
		}

		created, err := scaffold.Create(home.Path)
		if err != nil {
			return err
		}
		configFile, _, err := config.EnsureFile()
		if err != nil {
			return err
		}

		if created {
			termange.PrintSuccessf("Created contour store at %s\n", home.Path)
		} else {
			termange.PrintSuccessf("A store already exists at %s — left as it is.\n", home.Path)
		}
		termange.PrintInfof("Config file: %s\n", configFile)
		if !home.Explicit {
			termange.PrintInfof("Relocate it any time with `%s set-home <path>`.\n", config.Program)
		}
		if created {
			termange.PrintInfof("Next: run `%s list` to see what's inside, or edit the samples under rules/.\n", config.Program)
		}
		return nil
	},
}

var initLocal bool

func init() {
	initCmd.Flags().BoolVar(&initLocal, "local", false,
		"create a local store for the project in the working directory ("+store.DefaultOverlayDir+"/)")
	rootCmd.AddCommand(initCmd)
}

// runInitLocal scaffolds the project's local store. It deliberately never
// resolves the central store: resolveStore would create one, with its samples,
// as a side effect — and a project can use contour with local context alone.
func runInitLocal() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine the working directory: %w", err)
	}

	overlay, created, err := scaffold.CreateLocal(cwd)
	if err != nil {
		return err
	}
	shown := "./" + filepath.Base(overlay)

	if !created {
		termange.PrintSuccessf("This project already has a local store at %s — left as it is.\n", shown)
		return nil
	}

	termange.PrintSuccessf("Created a local store at %s\n", shown)
	termange.PrintInfof("  rules/       loaded eagerly into every session in this project\n")
	termange.PrintInfof("  skills/      listed on the menu, fetched when relevant\n")
	termange.PrintInfof("  knowledge/   listed on the menu, fetched when relevant\n")
	termange.PrintInfof("\nAnything you add is live at once — no profile needed. %s/README.md covers the formats.\n", shown)
	termange.PrintInfof("Commit %s/ so everyone on the project shares it.\n", shown)
	return nil
}

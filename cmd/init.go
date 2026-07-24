package cmd

import (
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/scaffold"
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
		"up explicitly — notably at a path you have pointed the environment " +
		"variable at.\n\n" +
		"init is safe to re-run: it creates whatever is missing and never " +
		"overwrites files you already have.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve rather than resolveStore: init is the command that creates
		// the store, so a missing directory is the normal case here — including
		// at an explicitly configured path, which resolveStore rejects.
		home, err := config.Resolve()
		if err != nil {
			return err
		}
		fresh := !home.Exists

		if err := scaffold.Create(home.Path); err != nil {
			return err
		}
		configFile, _, err := config.EnsureFile()
		if err != nil {
			return err
		}

		if fresh {
			termange.PrintSuccessf("Created contour store at %s\n", home.Path)
		} else {
			termange.PrintSuccessf("Store ready at %s (existing files left untouched)\n", home.Path)
		}
		termange.PrintInfof("Config file: %s\n", configFile)
		if !home.Explicit {
			termange.PrintInfof("Relocate it any time with `%s set-home <path>`.\n", config.Program)
		}
		termange.PrintInfof("Next: run `%s list` to see what's inside, or edit the samples under rules/.\n", config.Program)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

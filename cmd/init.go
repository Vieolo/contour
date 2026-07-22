package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/termange"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the contour store skeleton",
	Long: "Create the contour store directory and its folder structure " +
		"(bootstrap, rules, skills, knowledge), a README describing the " +
		"convention, and a few sample files.\n\n" +
		"init is safe to re-run: it creates whatever is missing and never " +
		"overwrites files you already have.",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := config.Resolve()
		if err != nil {
			return err
		}
		return runInit(home)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(home config.Home) error {
	fresh := !home.Exists

	// Ensure the root and every kind/bootstrap directory exists, so even a
	// store with no seed files has the expected shape.
	dirs := []string{home.Path, filepath.Join(home.Path, store.BootstrapDir)}
	for _, k := range store.Kinds {
		dirs = append(dirs, filepath.Join(home.Path, string(k)))
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	// Write the sample files, leaving any that already exist untouched.
	for _, f := range seedFiles() {
		path := filepath.Join(home.Path, filepath.FromSlash(f.rel))
		if err := writeIfAbsent(path, f.body); err != nil {
			return err
		}
	}

	if fresh {
		termange.PrintSuccessf("Created contour store at %s\n", home.Path)
	} else {
		termange.PrintSuccessf("Store ready at %s (existing files left untouched)\n", home.Path)
	}
	if !home.Explicit {
		termange.PrintInfof("Relocate it any time by setting %s to a new path.\n", config.EnvVar)
	}
	termange.PrintInfof("Next: run `%s list` to see what's inside, or edit the samples under rules/.\n", config.Program)
	return nil
}

// writeIfAbsent writes body to path only when path does not already exist,
// creating parent directories as needed. Existing files are never overwritten.
func writeIfAbsent(path, body string) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return nil // already present
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

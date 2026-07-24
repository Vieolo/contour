package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/scaffold"
	"github.com/vieolo/termange"
)

var setHomeCmd = &cobra.Command{
	Use:   "set-home <path>",
	Short: "Move the contour store to another directory",
	Long: "Move the store to a new directory and record the choice in contour's " +
		"config file.\n\n" +
		"Your content comes with it. Leaving the store behind and creating an " +
		"empty one at the destination would only hand you a manual merge, with a " +
		"name clash on every sample file.\n\n" +
		"What happens depends on what is there:\n" +
		"  - an existing store, with the destination free — the store is moved\n" +
		"  - no store yet — a new one is created at the destination\n" +
		"  - the destination already has content — it is adopted as-is, and " +
		"nothing is moved or overwritten\n\n" +
		"The config file lives outside the store, so the setting survives the " +
		"move. Being a file rather than an environment variable, it is also " +
		"picked up when an agent launches contour as an MCP server — such a " +
		"process does not inherit your shell.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := config.NormalizePath(args[0])
		if err != nil {
			return err
		}
		current, err := config.Resolve()
		if err != nil {
			return err
		}

		if current.Path == target {
			return confirmAlreadyThere(current, target)
		}

		occupied, err := dirHasContent(target)
		if err != nil {
			return err
		}

		// Do the disk work before recording anything, so a failure never leaves
		// the config pointing at a store that isn't there.
		var summary string
		switch {
		case occupied:
			// Most likely a store the user moved by hand. Adopt it rather than
			// risk writing over content that is already there.
			summary = fmt.Sprintf("Pointed contour at the existing directory %s", target)

		case current.Exists:
			if err := removeEmptyDir(target); err != nil {
				return err
			}
			if err := scaffold.Move(current.Path, target); err != nil {
				return err
			}
			summary = fmt.Sprintf("Moved your store from %s to %s", current.Path, target)

		default:
			if err := scaffold.Create(target); err != nil {
				return err
			}
			summary = fmt.Sprintf("Created a new store at %s", target)
		}

		_, configFile, err := config.SetStorePath(target)
		if err != nil {
			return err
		}

		termange.PrintSuccessf("%s\n", summary)
		termange.PrintInfof("Recorded in %s\n", configFile)
		if occupied && current.Exists {
			termange.PrintWarningf("\nYour previous store is still at %s — nothing was moved or overwritten.\n", current.Path)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setHomeCmd)
}

// confirmAlreadyThere handles pointing the store at where it already is, which
// is worth supporting: it is how someone repairs a config that has drifted, or
// pins the default location explicitly.
func confirmAlreadyThere(current config.Home, target string) error {
	if !current.Exists {
		if err := scaffold.Create(target); err != nil {
			return err
		}
	}
	_, configFile, err := config.SetStorePath(target)
	if err != nil {
		return err
	}

	termange.PrintSuccessf("Store is already at %s\n", target)
	termange.PrintInfof("Recorded in %s\n", configFile)
	return nil
}

// dirHasContent reports whether path exists and holds anything at all.
func dirHasContent(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s already exists and is not a directory", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	return len(entries) > 0, nil
}

// removeEmptyDir clears an empty destination so a rename can take its place.
// Callers reach it only once the destination is known to be empty.
func removeEmptyDir(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove empty %s: %w", path, err)
	}
	return nil
}

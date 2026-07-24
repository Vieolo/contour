package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/scaffold"
	"github.com/vieolo/termange"
)

// hereKeyword puts the store in the working directory without typing a path.
const hereKeyword = "here"

var setHomeCmd = &cobra.Command{
	Use:   "set-home <path|here>",
	Short: "Move the contour store to another directory",
	Long: "Move the store to a new directory and record the choice in contour's " +
		"config file.\n\n" +
		"Your content comes with it. Leaving the store behind and creating an " +
		"empty one at the destination would only hand you a manual merge, with a " +
		"name clash on every sample file.\n\n" +
		"Pass `here` instead of a path to put the store in the directory you are " +
		"standing in. It creates a folder for the store rather than filling the " +
		"working directory itself, so running it from ~/Documents gives you " +
		"~/Documents/contour and leaves your documents alone. To target a " +
		"directory that is genuinely named \"here\", write ./here.\n\n" +
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
		target, err := resolveTarget(args[0])
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
			if err := refuseMoveIntoItself(current.Path, target); err != nil {
				return err
			}
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

// resolveTarget turns the path argument into an absolute destination, expanding
// the "here" shorthand to a store directory inside the working directory.
//
// "here" deliberately creates a subdirectory rather than using the working
// directory itself: someone running it from ~/Documents wants a store alongside
// their other folders, not their documents turned into one. The name matches the
// default store directory, so a dev build lands beside a production store rather
// than on top of it.
//
// Only the bare word is treated as the keyword, leaving ./here to address a
// directory actually named "here".
func resolveTarget(arg string) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(arg), hereKeyword) {
		return config.NormalizePath(arg)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine the working directory: %w", err)
	}
	return config.NormalizePath(filepath.Join(cwd, config.StoreDirName()))
}

// refuseMoveIntoItself rejects a destination inside the store being moved — most
// easily reached by running `set-home here` from within the store. A rename
// rejects it outright, but the cross-filesystem copy fallback would descend into
// its own output and never finish.
func refuseMoveIntoItself(store, target string) error {
	if target == store || strings.HasPrefix(target, store+string(filepath.Separator)) {
		return fmt.Errorf("cannot move the store inside itself: %s is within %s", target, store)
	}
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

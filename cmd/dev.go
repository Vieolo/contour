//go:build dev

// This file is compiled only into development builds (`go build -tags dev`).
// The commands it registers — seed and nuke — operate on the development store
// and never exist in the production binary.
package cmd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/termange"
)

var (
	seedForce bool
	nukeForce bool
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Copy the production store into the development store",
	Long: "Copy the production store (CONTOUR_HOME) into the development store " +
		"(CONTOUR_HOME_DEV) so you can test against real content without " +
		"touching production.\n\n" +
		"If the development store already exists, pass --force to replace it. " +
		"This command exists only in development builds.",
	RunE: runSeed,
}

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Delete the development store",
	Long: "Permanently delete the development store (CONTOUR_HOME_DEV) so you " +
		"can test a clean initialization.\n\n" +
		"Requires --force to confirm. This command exists only in development " +
		"builds.",
	RunE: runNuke,
}

func init() {
	seedCmd.Flags().BoolVarP(&seedForce, "force", "f", false, "replace the development store if it already exists")
	nukeCmd.Flags().BoolVarP(&nukeForce, "force", "f", false, "confirm permanent deletion of the development store")
	rootCmd.AddCommand(seedCmd, nukeCmd)
}

func runSeed(cmd *cobra.Command, args []string) error {
	src, err := config.ResolveProduction()
	if err != nil {
		return err
	}
	dst, err := config.Resolve() // the development store
	if err != nil {
		return err
	}

	if src.Path == dst.Path {
		return fmt.Errorf("production and development stores resolve to the same path (%s); refusing to seed", src.Path)
	}
	if !src.Exists {
		return fmt.Errorf("production store not found at %s; nothing to seed from", src.Path)
	}
	if dst.Exists {
		if !seedForce {
			return fmt.Errorf("development store already exists at %s; re-run with --force to replace it, or run `%s nuke` first", dst.Path, config.Program)
		}
		if err := os.RemoveAll(dst.Path); err != nil {
			return fmt.Errorf("remove existing development store: %w", err)
		}
	}

	if err := copyTree(src.Path, dst.Path); err != nil {
		return err
	}

	termange.PrintSuccessf("Seeded development store at %s\n", dst.Path)
	termange.PrintInfof("Source: %s\n", src.Path)
	return nil
}

func runNuke(cmd *cobra.Command, args []string) error {
	dst, err := config.Resolve() // the development store
	if err != nil {
		return err
	}
	prod, err := config.ResolveProduction()
	if err != nil {
		return err
	}

	// Safety net: never delete the production store, even if CONTOUR_HOME_DEV
	// has been pointed at it by mistake.
	if dst.Path == prod.Path {
		return fmt.Errorf("development store resolves to the production path (%s); refusing to nuke", dst.Path)
	}
	if !dst.Exists {
		termange.PrintInfof("Nothing to remove: %s does not exist.\n", dst.Path)
		return nil
	}
	if !nukeForce {
		return fmt.Errorf("this will permanently delete %s; re-run with --force to confirm", dst.Path)
	}

	if err := os.RemoveAll(dst.Path); err != nil {
		return fmt.Errorf("remove development store: %w", err)
	}

	termange.PrintSuccessf("Removed development store at %s\n", dst.Path)
	return nil
}

// copyTree recursively copies the directory tree rooted at src into dst,
// creating dst and any parents. Non-regular files (symlinks, sockets) are
// skipped.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

// copyFile copies a single regular file, preserving its permission bits and
// creating the destination's parent directories as needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	return nil
}

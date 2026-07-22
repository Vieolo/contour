package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/termange"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show an overview of the contour store",
	Long: "Show how many items of each kind the store currently holds.\n\n" +
		"This overview is deliberately minimal; richer listing (descriptions " +
		"and tags) arrives with the loader.",
	RunE: func(cmd *cobra.Command, args []string) error {
		home := resolveStore()

		if config.Dev {
			termange.PrintWarningln(config.Label + " build — using the dev store")
		}
		termange.PrintInfof("contour store: %s\n\n", home.Path)
		for _, k := range store.Kinds {
			n, err := countItems(filepath.Join(home.Path, string(k)), k)
			if err != nil {
				return err
			}
			termange.PrintInfof("  %-10s %d\n", string(k), n)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

// countItems counts the items under a kind directory: SKILL.md files for
// skills, markdown files otherwise. A missing kind directory counts as zero.
//
// This is a stopgap until the loader lands and can report items with their
// descriptions and tags.
func countItems(dir string, kind store.Kind) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil // kind directory not created yet
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if kind == store.KindSkills {
			if d.Name() == store.SkillFile {
				count++
			}
			return nil
		}
		if filepath.Ext(d.Name()) == store.MarkdownExt {
			count++
		}
		return nil
	})
	return count, err
}

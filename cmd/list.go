package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/termange"
)

var listCmd = &cobra.Command{
	Use:   "list [kind]",
	Short: "List the items in the contour store",
	Long: "List the store's items with their tags and descriptions. Optionally " +
		"restrict to a single kind: rules, skills or knowledge.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home := resolveStore()
		st, err := store.Load(home.Path)
		if err != nil {
			return err
		}

		kinds := store.Kinds
		if len(args) == 1 {
			k, err := parseKind(args[0])
			if err != nil {
				return err
			}
			kinds = []store.Kind{k}
		}

		if config.Dev {
			termange.PrintWarningln(config.Label + " build — using the dev store")
		}
		termange.PrintInfof("contour store: %s\n", home.Path)

		for _, k := range kinds {
			items := st.ByKind(k)
			fmt.Println()
			termange.PrintColorf(termange.ColorGreen, "%s (%d)\n", string(k), len(items))
			if len(items) == 0 {
				termange.PrintInfoln("  (none)")
				continue
			}
			for _, it := range items {
				printItem(it)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func printItem(it store.Item) {
	termange.PrintInfof("  %s\n", it.ID)
	if it.Description != "" {
		termange.PrintInfof("      %s\n", it.Description)
	}
	if len(it.Tags) > 0 {
		termange.PrintColorf(termange.ColorYellow, "      tags: %s\n", strings.Join(it.Tags, ", "))
	}
}

// parseKind maps a user-supplied kind argument to a store.Kind.
func parseKind(arg string) (store.Kind, error) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "rules", "rule":
		return store.KindRules, nil
	case "skills", "skill":
		return store.KindSkills, nil
	case "knowledge":
		return store.KindKnowledge, nil
	default:
		return "", fmt.Errorf("unknown kind %q (want: rules, skills or knowledge)", arg)
	}
}

package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/scaffold"
	"github.com/vieolo/termange"
)

var setHomeCmd = &cobra.Command{
	Use:   "set-home <path>",
	Short: "Change where the contour store lives",
	Long: "Point contour at a different store directory and record the choice in " +
		"its config file.\n\n" +
		"The config file lives outside the store, so the setting survives moving " +
		"the store itself. Being a file rather than an environment variable, it is " +
		"also picked up when an agent launches contour as an MCP server — such a " +
		"process does not inherit your shell.\n\n" +
		"If the directory does not exist it is created with the standard store " +
		"structure. To relocate an existing store, move it first and then point " +
		"contour at its new home:\n\n" +
		"    mv ~/contour /new/path\n" +
		"    contour set-home /new/path",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storePath, configFile, err := config.SetStorePath(args[0])
		if err != nil {
			return err
		}

		info, statErr := os.Stat(storePath)
		fresh := statErr != nil || !info.IsDir()

		// scaffold never overwrites existing files, so pointing at a store that
		// is already populated leaves its content untouched.
		if err := scaffold.Create(storePath); err != nil {
			return err
		}

		if fresh {
			termange.PrintSuccessf("Created a new store at %s\n", storePath)
		} else {
			termange.PrintSuccessf("Store location set to %s\n", storePath)
		}
		termange.PrintInfof("Recorded in %s\n", configFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setHomeCmd)
}

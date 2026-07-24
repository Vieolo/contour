package cmd

import (
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/termange"
)

var homeCmd = &cobra.Command{
	Use:   "home",
	Short: "Show where the contour store lives",
	Long: "Print the store's location, how contour decided on it, and the config " +
		"file that records it.\n\n" +
		"Use `contour set-home <path>` to change the location.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve rather than resolveStore: reporting where the store would be
		// should never create one as a side effect.
		home, err := config.Resolve()
		if err != nil {
			return err
		}
		configFile, err := config.ConfigPath()
		if err != nil {
			return err
		}

		termange.PrintInfof("store:  %s\n", home.Path)
		termange.PrintInfof("source: %s\n", string(home.Source))
		termange.PrintInfof("config: %s\n", configFile)

		if home.Source == config.SourceEnv {
			termange.PrintWarningf("\n%s is set, so it overrides the config file.\n", config.EnvVar)
		}
		if !home.Exists {
			termange.PrintWarningln("\nThat directory does not exist yet; it is created when a command needs it.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(homeCmd)
}

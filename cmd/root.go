package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "contour",
	Short: "centralized context provider",
	Long:  `contour is the centralized context provider for your AI agent`,
	// Errors are reported by the commands themselves; don't append usage text.
	SilenceUsage: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
//
// Version-aware setup runs here (not in init) because main.go injects
// ThisGyByte after package init has finished.
func Execute() {

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

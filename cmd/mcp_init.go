package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpconfig"
	"github.com/vieolo/termange"
)

var (
	mcpInitProfile string
	mcpInitFile    string
	mcpInitName    string
)

var mcpInitCmd = &cobra.Command{
	Use:   "mcp-init",
	Short: "Register contour as an MCP server in this project",
	Long: "Write this project's MCP config so an agent starts contour for you, " +
		"creating " + mcpconfig.DefaultFile + " or adding contour to the one " +
		"already there. Other servers in the file are left untouched.\n\n" +
		"The entry records contour's absolute path rather than the bare command " +
		"name. An agent launches its servers without a login shell, so it does " +
		"not get the PATH that would resolve `contour` — a Homebrew install " +
		"lives outside the system default PATH entirely.\n\n" +
		"Pass --bootstrap to pin the profile whose rules load at the start of " +
		"every session in this project. The profile is checked against your " +
		"store, so a typo is caught here rather than at session start.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The path contour was launched from. Deliberately not resolved through
		// symlinks: Homebrew's bin entry points into a versioned Cellar
		// directory, and recording that would break on the next upgrade.
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determine contour's own path: %w", err)
		}

		serverArgs := []string{"mcp"}
		if mcpInitProfile != "" {
			if err := checkProfileExists(mcpInitProfile); err != nil {
				return err
			}
			serverArgs = append(serverArgs, "--bootstrap", mcpInitProfile)
		}

		replaced, err := mcpconfig.Upsert(mcpInitFile, mcpInitName, mcpconfig.Entry{
			Command: exe,
			Args:    serverArgs,
		})
		if err != nil {
			return err
		}

		if replaced {
			termange.PrintSuccessf("Updated the %q server in %s\n", mcpInitName, mcpInitFile)
		} else {
			termange.PrintSuccessf("Added the %q server to %s\n", mcpInitName, mcpInitFile)
		}
		termange.PrintInfof("  command: %s\n", exe)

		if mcpInitProfile == "" {
			termange.PrintWarningln("\nNo bootstrap profile pinned, so no rules load automatically.")
			termange.PrintWarningf("Re-run with --bootstrap <name> to choose one (%s bootstrap lists them).\n", config.Program)
		} else {
			termange.PrintInfof("  profile: %s\n", mcpInitProfile)
		}
		return nil
	},
}

func init() {
	mcpInitCmd.Flags().StringVar(&mcpInitProfile, "bootstrap", "",
		"bootstrap profile whose rules load eagerly in this project")
	mcpInitCmd.Flags().StringVar(&mcpInitFile, "file", mcpconfig.DefaultFile,
		"MCP config file to create or update")
	mcpInitCmd.Flags().StringVar(&mcpInitName, "name", config.Program,
		"name to register the server under")
	rootCmd.AddCommand(mcpInitCmd)
}

// checkProfileExists rejects an unknown profile now rather than leaving the
// agent to start a session with nothing loaded.
func checkProfileExists(name string) error {
	home, err := resolveStore()
	if err != nil {
		return err
	}
	if _, err := bootstrap.LoadProfile(home.Path, name); err != nil {
		return withAvailableProfiles(home.Path, err)
	}
	return nil
}

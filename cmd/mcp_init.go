package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpconfig"
	"github.com/vieolo/contour/internal/project"
	"github.com/vieolo/termange"
)

var (
	mcpInitProfile string
	mcpInitFile    string
	mcpInitName    string
)

// commonAgentFiles are the single-file conventions mcp-init offers to load
// eagerly, so a project that already keeps one need not restructure it.
var commonAgentFiles = []string{"AGENTS.md", "AGENT.md", "CLAUDE.md"}

var mcpInitCmd = &cobra.Command{
	Use:   "mcp-init",
	Short: "Register contour as an MCP server in this project",
	Long: "Set this project up for contour: write the MCP config so an agent " +
		"starts contour, and a project config that pins the profile and lists any " +
		"files to load eagerly.\n\n" +
		"The MCP entry (in " + mcpconfig.DefaultFile + ") records contour's " +
		"absolute path — an agent launches its servers without a login shell, so a " +
		"bare `contour` would not resolve — and a bare `mcp` command, because the " +
		"profile now lives in the project config rather than the launch arguments.\n\n" +
		"Pass --bootstrap to set the profile. Any AGENTS.md or CLAUDE.md at the " +
		"project root is detected and listed for eager loading; edit the config to " +
		"change that.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The path contour was launched from. Deliberately not resolved through
		// symlinks: Homebrew's bin entry points into a versioned Cellar
		// directory, and recording that would break on the next upgrade.
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determine contour's own path: %w", err)
		}
		if mcpInitProfile != "" {
			if err := checkProfileExists(mcpInitProfile); err != nil {
				return err
			}
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("determine the working directory: %w", err)
		}
		eager := detectEagerFiles(cwd)

		// 1. The project config carries the profile and eager files, so the
		//    settings live with the project and .mcp.json stays a bare launch.
		cfgPath, err := project.Write(cwd, mcpInitProfile, eager)
		if err != nil {
			return err
		}

		// 2. Register the server with a bare `mcp`; the profile comes from the
		//    config written above.
		replaced, err := mcpconfig.Upsert(mcpInitFile, mcpInitName, mcpconfig.Entry{
			Command: exe,
			Args:    []string{"mcp"},
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
		termange.PrintInfof("  config:  %s\n", cfgPath)
		if mcpInitProfile != "" {
			termange.PrintInfof("  profile: %s\n", mcpInitProfile)
		}
		for _, f := range eager {
			termange.PrintInfof("  eager:   %s (detected)\n", f)
		}
		if mcpInitProfile == "" {
			termange.PrintWarningf("\nNo profile set — add `bootstrap: <name>` to %s (%s bootstrap lists them).\n", cfgPath, config.Program)
		}
		return nil
	},
}

func init() {
	mcpInitCmd.Flags().StringVar(&mcpInitProfile, "bootstrap", "",
		"bootstrap profile to record in the project config")
	mcpInitCmd.Flags().StringVar(&mcpInitFile, "file", mcpconfig.DefaultFile,
		"MCP config file to create or update")
	mcpInitCmd.Flags().StringVar(&mcpInitName, "name", config.Program,
		"name to register the server under")
	rootCmd.AddCommand(mcpInitCmd)
}

// detectEagerFiles finds common agent-instruction files at the project root, so
// mcp-init can list them for eager loading rather than making the user restate
// what they already keep.
func detectEagerFiles(projectDir string) []string {
	var found []string
	for _, name := range commonAgentFiles {
		if info, err := os.Stat(filepath.Join(projectDir, name)); err == nil && !info.IsDir() {
			found = append(found, name)
		}
	}
	return found
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

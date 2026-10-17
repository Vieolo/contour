package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpconfig"
	"github.com/vieolo/contour/internal/project"
	"github.com/vieolo/termange"
)

var (
	mcpInitProfiles []string
	mcpInitFile     string
	mcpInitName     string
)

// commonAgentFiles are the single-file conventions mcp-init offers to load
// eagerly, so a project that already keeps one need not restructure it.
var commonAgentFiles = []string{"AGENTS.md", "AGENT.md", "CLAUDE.md"}

var mcpInitCmd = &cobra.Command{
	Use:   "mcp-init",
	Short: "Register contour as an MCP server in this project",
	Long: "Register contour as an MCP server for this project on this machine, " +
		"and give the project a contour config if it has none.\n\n" +
		"The MCP entry (in " + mcpconfig.DefaultFile + ") is always written. It " +
		"records contour's absolute path — an agent launches its servers without " +
		"a login shell, so a bare `contour` would not resolve — which makes it " +
		"specific to this machine: keep " + mcpconfig.DefaultFile + " out of git, " +
		"and have each developer run this once.\n\n" +
		"The project config pins the profiles and lists any files to load " +
		"eagerly. It goes in the project's go.yaml, under external.contour, if it " +
		"has one — a Go project need not carry a second config file — and " +
		"otherwise in .contour.yaml at the project root. It is written only when " +
		"the project has none: an existing config is the team's committed setup, " +
		"and is never modified here. To change it, edit the file.\n\n" +
		"Pass --bootstrap to choose the profiles for a new config, repeating it to " +
		"combine entry points (--bootstrap python --bootstrap cli). Any AGENTS.md " +
		"or CLAUDE.md at the project root is detected and listed for eager " +
		"loading. Against an existing config, --bootstrap is accepted only if it " +
		"names the profiles the config already selects.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The path contour was launched from. Deliberately not resolved through
		// symlinks: Homebrew's bin entry points into a versioned Cellar
		// directory, and recording that would break on the next upgrade.
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determine contour's own path: %w", err)
		}
		if err := checkProfilesExist(mcpInitProfiles); err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("determine the working directory: %w", err)
		}
		// 1. Give the project a config if it has none. An existing one is the
		//    team's committed setup and is left alone — every developer runs
		//    this command to register contour on their own machine, so it must
		//    not be able to rewrite shared settings. Settled before .mcp.json is
		//    touched, so a conflict leaves nothing half-done.
		cfg, created, err := project.Ensure(cwd, mcpInitProfiles, detectEagerFiles(cwd))
		if errors.Is(err, project.ErrProfileConflict) {
			return fmt.Errorf("%s already exists (bootstrap: [%s]) and mcp-init never changes an existing project config — "+
				"edit that file to switch profiles, or run without --bootstrap to just register contour on this machine",
				cfg.Path, strings.Join(cfg.Bootstrap, ", "))
		}
		if err != nil {
			return err
		}
		for _, w := range cfg.Warnings {
			fmt.Fprintf(os.Stderr, "contour: %s\n", w)
		}

		// 2. Register the server with a bare `mcp`; the profiles come from the
		//    project config, not the launch arguments.
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
		if created {
			termange.PrintInfof("  config:  %s (created)\n", cfg.Path)
		} else {
			termange.PrintInfof("  config:  %s (existing, left unchanged)\n", cfg.Path)
		}
		for _, p := range cfg.Bootstrap {
			termange.PrintInfof("  profile: %s\n", p)
		}
		for _, f := range cfg.EagerFiles {
			termange.PrintInfof("  eager:   %s\n", f.ID)
		}

		if len(cfg.Bootstrap) == 0 {
			termange.PrintWarningf("\nNo profile set — add `bootstrap: [<name>]` to %s (%s bootstrap lists them).\n", cfg.Path, config.Program)
		} else if !created {
			// The config came from the team, but the store is this machine's. A
			// profile missing here makes the MCP server fail to start, so say so
			// now, at setup, rather than when an agent first connects.
			if err := checkProfilesExist(cfg.Bootstrap); err != nil {
				termange.PrintWarningf("\nThis project's profiles are not all in your store: %v\n"+
					"The MCP server will not start until they are.\n", err)
			}
		}
		return nil
	},
}

func init() {
	mcpInitCmd.Flags().StringSliceVar(&mcpInitProfiles, "bootstrap", nil,
		"bootstrap profile for a new project config (repeatable)")
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

// checkProfilesExist rejects an unknown profile now rather than leaving the
// agent to start a session with nothing loaded.
func checkProfilesExist(names []string) error {
	if len(names) == 0 {
		return nil
	}
	home, err := resolveStore()
	if err != nil {
		return err
	}
	if _, err := bootstrap.LoadNamed(home.Path, names); err != nil {
		return withAvailableProfiles(home.Path, err)
	}
	return nil
}

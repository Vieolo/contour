package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/render"
	"github.com/vieolo/contour/internal/scaffold"
	"github.com/vieolo/contour/internal/unic"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "contour",
	Short: "centralized context provider",
	Long:  `contour is the centralized context provider for your AI agent`,
	// Errors are reported by the commands themselves; don't append usage text.
	SilenceUsage: true,
}

var mcpCommands []unic.Command

// Adds a command to:
//  1. cobra's rootCmd, immediately
//  2. mcp server, when the MCP server is started
//
// This function is used for dual-use commands (such as `list`)
// and CLI-only commands don't use this (such as `nuke`)
func addCommand(c unic.Command) {
	if cli := c.GetCLIConfig(); cli != nil {
		rootCmd.AddCommand(cli)
	}
	mcpCommands = append(mcpCommands, c)
}

// resolveStore locates the contour store for commands that read from it.
//
// The store is a single central directory set up once, not a per-project
// scaffold, so a missing default location is not a failure: contour creates the
// store, explains its layout, and carries on. An explicitly configured location
// that does not exist is a different matter — that is a misconfiguration, and
// silently creating a store somewhere the user did not expect would be worse
// than saying so.
//
// The notice goes to stderr, keeping stdout free for output that may be piped
// into an agent.
func resolveStore() (config.Home, error) {
	home, err := config.Resolve()
	if err != nil {
		return config.Home{}, err
	}
	if home.Exists {
		return home, nil
	}

	if home.Explicit {
		return config.Home{}, fmt.Errorf(
			"%s points to %q, but that directory does not exist; create it, point %s at your store, or unset it to use the default",
			config.EnvVar, home.Path, config.EnvVar)
	}

	if err := scaffold.Create(home.Path); err != nil {
		return config.Home{}, err
	}
	render.StoreCreated(home.Path)

	home.Exists = true
	return home, nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

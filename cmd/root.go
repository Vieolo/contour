package cmd

import (
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
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

// dualCommands holds the commands that expose both a CLI and an MCP surface, in
// registration order.
var dualCommands []unic.Command

// addCommand wires a dual-surface command into both surfaces at once: the cobra
// tree immediately, and the MCP server when `contour mcp` builds one. Commands
// call it from their init, so adding a command cannot leave one surface behind.
//
// CLI-only commands (init, version, seed, nuke, …) skip this and call
// rootCmd.AddCommand directly.
func addCommand(c unic.Command) {
	if cli := c.GetCLIConfig(); cli != nil {
		rootCmd.AddCommand(cli)
	}
	dualCommands = append(dualCommands, c)
}

// registerMCPTools attaches every dual-surface command's tool to the server.
// Commands without an MCP handler register nothing.
func registerMCPTools(s *mcp.Server) {
	for _, c := range dualCommands {
		c.RegisterMCPTool(s)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

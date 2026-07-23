package cmd

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vieolo/contour/internal/unic"
)

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

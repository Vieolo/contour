// Package unic defines a contour command once for both surfaces it can be
// reached through: the cobra CLI and the MCP server.
//
// The two handlers stay separate on purpose. A CLI run writes colourised,
// human-scannable output to a terminal; a tool call returns plain text sized for
// a model's context. Sharing the definition keeps the surfaces from drifting,
// while sharing a handler would make both worse.
//
// Commands that only ever run in the terminal stay plain *cobra.Command. The
// difference in type is deliberate: opening a file under cmd/ tells you at a
// glance whether it is CLI-only or dual-surface.
package unic

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// Command is the surface-agnostic view of a UniversalCommand. It lets commands
// with different input types be collected and registered together.
type Command interface {
	GetCLIConfig() *cobra.Command
	RegisterMCPTool(s *mcp.Server)
}

// UniversalCommand is one command expressed for both surfaces. In is the tool's
// input struct — the MCP schema is inferred from it — and Out is the structured
// output type, usually any to omit an output schema.
//
// Either handler may be nil: without MCPCommand the command is CLI-only,
// without CLICommand it is reachable only over MCP.
type UniversalCommand[In, Out any] struct {
	// Use is the cobra usage line, e.g. "list [kind]". Its first word doubles
	// as the MCP tool name unless Name overrides it.
	Use string

	// Name overrides the MCP tool name. Leave it empty to use the first word of
	// Use, the same rule cobra applies to derive a command's name.
	Name string

	// Short and Long are cobra's help text, written for a person.
	Short string
	Long  string

	// Description is the MCP tool description. It is effectively a prompt: the
	// model reads it to decide whether to call the tool, so it is worth writing
	// separately from Short. Falls back to Short when empty.
	Description string

	// Args validates positional CLI arguments.
	Args cobra.PositionalArgs

	// CLIFlags registers flags on the cobra command. It runs for the CLI surface
	// only, which is the point: an option that serves a person curating the store
	// has no business in a tool schema, where it would be one more thing for a
	// model to consider and get wrong.
	CLIFlags func(cmd *cobra.Command)

	CLICommand func(cmd *cobra.Command, args []string) error
	MCPCommand mcp.ToolHandlerFor[In, Out]
}

// ToolName returns the MCP tool name: Name when set, otherwise the first word
// of Use.
func (c *UniversalCommand[In, Out]) ToolName() string {
	if c.Name != "" {
		return c.Name
	}
	if i := strings.IndexAny(c.Use, " \t"); i > 0 {
		return c.Use[:i]
	}
	return c.Use
}

// GetCLIConfig builds the cobra command, returning nil when the command has no
// CLI surface.
func (c *UniversalCommand[In, Out]) GetCLIConfig() *cobra.Command {
	if c.CLICommand == nil {
		return nil
	}
	cmd := &cobra.Command{
		Use:   c.Use,
		Short: c.Short,
		Long:  c.Long,
		Args:  c.Args,
		RunE:  c.CLICommand,
	}
	if c.CLIFlags != nil {
		c.CLIFlags(cmd)
	}
	return cmd
}

// RegisterMCPTool adds the command to an MCP server. It is a no-op for CLI-only
// commands, so callers can register every command unconditionally rather than
// maintaining a list of which ones have tools.
func (c *UniversalCommand[In, Out]) RegisterMCPTool(s *mcp.Server) {
	if c.MCPCommand == nil {
		return
	}

	description := c.Description
	if description == "" {
		description = c.Short
	}

	mcp.AddTool[In, Out](s, &mcp.Tool{
		Name:        c.ToolName(),
		Description: description,
	}, c.MCPCommand)
}

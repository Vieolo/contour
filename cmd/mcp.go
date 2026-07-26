package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/usage"
)

var mcpBootstrap string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the contour MCP server over stdio",
	Long: "Serve the contour store to an MCP client over stdio.\n\n" +
		"The selected bootstrap profile's rules are delivered eagerly in the " +
		"server's instructions, while skills and knowledge are reachable " +
		"through the list, search and get tools for on-demand fetching.\n\n" +
		"Select the profile with --bootstrap. Clients configure it per project — " +
		"in Claude Code's .mcp.json, for instance, it goes in the server's args. " +
		"Without a profile the server still serves the whole store through its " +
		"tools, and says how to choose an entry point.\n\n" +
		"The store's location comes from contour's config file, so the server " +
		"finds it even though an agent launches it without your shell.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// resolveStore writes any notice to stderr, so stdout stays free for
		// the MCP protocol.
		home, err := resolveStore()
		if err != nil {
			return err
		}

		if config.Dev {
			fmt.Fprintf(os.Stderr, "[%s build] contour mcp: store=%s profile=%q\n",
				config.Label, home.Path, mcpBootstrap)
		}

		// Open the session's usage log, if enabled. It is best-effort: a failure
		// to open (or a disabled toggle) leaves mcpUsage nil, and its nil-safe
		// methods make the handlers no-op — logging never blocks serving.
		if enabled, err := config.UsageLoggingEnabled(); err == nil && enabled {
			if logger, err := usage.Open(mcpBootstrap); err != nil {
				fmt.Fprintf(os.Stderr, "contour: usage logging off (%v)\n", err)
			} else {
				mcpUsage = logger
				defer mcpUsage.Close()
			}
		}

		server, err := mcpserver.New(mcpserver.Options{
			Root:     home.Path,
			Overlays: projectOverlays(),
			Profile:  mcpBootstrap,
			Version:  cliVersion(),
		})
		if err != nil {
			return err
		}

		// Registering the dual-usage commands from `mcpCommands`
		// The commands are automatically added to `mcpCommands` via
		// the `addCommand` function triggered in their `init` function
		for _, c := range mcpCommands {
			c.RegisterMCPTool(server)
		}

		// Shut down cleanly when the host stops us.
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return mcpserver.Serve(ctx, server)
	},
}

func init() {
	mcpCmd.Flags().StringVar(&mcpBootstrap, "bootstrap", "",
		"bootstrap profile whose rules are loaded eagerly")
	rootCmd.AddCommand(mcpCmd)
}

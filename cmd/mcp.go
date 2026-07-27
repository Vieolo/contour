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

var mcpBootstrap []string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the contour MCP server over stdio",
	Long: "Serve the contour store to an MCP client over stdio.\n\n" +
		"The selected bootstrap profiles' rules are delivered eagerly in the " +
		"server's instructions, while skills and knowledge are reachable " +
		"through the list, search and get tools for on-demand fetching.\n\n" +
		"The profiles normally come from the project config; --bootstrap " +
		"overrides them and may be repeated to combine entry points. Without " +
		"any profile the server still serves the whole store through its " +
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

		// The --bootstrap flag wins; otherwise the project config chooses the
		// profiles, so they can live with the project instead of in .mcp.json.
		overlays, eagerFiles, cfgBootstrap := projectContext()
		profiles := mcpBootstrap
		if len(profiles) == 0 {
			profiles = cfgBootstrap
		}
		// Publish them for the bootstrap tool, which serves this session's
		// payload rather than accepting profile names from the agent.
		mcpProfiles = profiles

		if config.Dev {
			fmt.Fprintf(os.Stderr, "[%s build] contour mcp: store=%s profiles=%q\n",
				config.Label, home.Path, profiles)
		}

		// Open the session's usage log, if enabled. It is best-effort: a failure
		// to open (or a disabled toggle) leaves mcpUsage nil, and its nil-safe
		// methods make the handlers no-op — logging never blocks serving.
		if enabled, err := config.UsageLoggingEnabled(); err == nil && enabled {
			if logger, err := usage.Open(profiles); err != nil {
				fmt.Fprintf(os.Stderr, "contour: usage logging off (%v)\n", err)
			} else {
				mcpUsage = logger
				defer mcpUsage.Close()
			}
		}

		server, err := mcpserver.New(mcpserver.Options{
			Root:       home.Path,
			Overlays:   overlays,
			EagerFiles: eagerFiles,
			Profiles:   profiles,
			Version:    cliVersion(),
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
	mcpCmd.Flags().StringSliceVar(&mcpBootstrap, "bootstrap", nil,
		"bootstrap profile whose rules are loaded eagerly (repeatable)")
	rootCmd.AddCommand(mcpCmd)
}

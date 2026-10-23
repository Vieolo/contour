package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/project"
	"github.com/vieolo/contour/internal/usage"
)

var (
	mcpBootstrap  []string
	mcpPresetFlag string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the contour MCP server over stdio",
	Long: "Serve the contour store to an MCP client over stdio.\n\n" +
		"The session starts with one of the project's presets: the one named by " +
		"--preset, or \"default\". Its profiles' rules and its eager files are " +
		"delivered eagerly in the server's instructions, while skills and " +
		"knowledge are reachable through the list, search and get tools for " +
		"on-demand fetching.\n\n" +
		"--preset belongs in a developer's own MCP config, to start every session " +
		"in the part of the project they work on. A name the project config does " +
		"not define falls back to default, and says so. --bootstrap replaces the " +
		"preset's profiles, keeping the rest of it. Without any profile the " +
		"server still serves the whole store through its tools.\n\n" +
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

		// The session starts with the preset named by --preset, or default. A
		// named preset the config lacks falls back to default, with a notice,
		// rather than refusing to start: the flag lives in a developer's own
		// .mcp.json, and a stale name there should cost them their preferred
		// starting point, not the whole server.
		overlays, cfg := projectContext()
		preset, found := cfg.Select(mcpPresetFlag)
		var notices []string
		if !found && mcpPresetFlag != "" {
			notices = append(notices, fmt.Sprintf("NOTE: this session was started with the preset %q, which this "+
				"project's config does not define, so it started with %q instead. Mention this to the user.",
				mcpPresetFlag, project.DefaultPreset))
			fmt.Fprintf(os.Stderr, "contour: no preset named %q in the project config; starting with %q\n",
				mcpPresetFlag, project.DefaultPreset)
			preset, _ = cfg.Select("")
		}
		mcpPreset = preset.Name

		// --bootstrap, an escape hatch, replaces the preset's profiles and
		// leaves the rest of the preset in force.
		profiles := preset.Bootstrap
		if len(mcpBootstrap) > 0 {
			profiles = mcpBootstrap
		}
		// Publish them for the bootstrap tool, which serves this session's
		// payload rather than accepting profile names from the agent.
		mcpProfiles = profiles

		if config.Dev {
			fmt.Fprintf(os.Stderr, "[%s build] contour mcp: store=%s preset=%q profiles=%q\n",
				config.Label, home.Path, preset.Name, profiles)
		}
		// The agent is told about missing profiles in its instructions; this is
		// for whoever reads the server's log.
		if _, missing, err := bootstrap.LoadAvailable(home.Path, profiles); err == nil && len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "contour: profile(s) not in your store, serving without them: %s\n", strings.Join(missing, ", "))
		}

		// Open the session's usage log, if enabled. It is best-effort: a failure
		// to open (or a disabled toggle) leaves mcpUsage nil, and its nil-safe
		// methods make the handlers no-op — logging never blocks serving.
		if enabled, err := config.UsageLoggingEnabled(); err == nil && enabled {
			if logger, err := usage.Open(preset.Name, profiles); err != nil {
				fmt.Fprintf(os.Stderr, "contour: usage logging off (%v)\n", err)
			} else {
				mcpUsage = logger
				defer mcpUsage.Close()
			}
		}

		server, err := mcpserver.New(mcpserver.Options{
			Root:       home.Path,
			Overlays:   overlays,
			EagerFiles: preset.EagerFiles,
			Profiles:   profiles,
			Notices:    notices,
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
	mcpCmd.Flags().StringVar(&mcpPresetFlag, "preset", "",
		"the project preset to start the session with (default: \"default\")")
	mcpCmd.Flags().StringSliceVar(&mcpBootstrap, "bootstrap", nil,
		"replace the preset's profiles with these (repeatable)")
	rootCmd.AddCommand(mcpCmd)
}

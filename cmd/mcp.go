package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpserver"
)

// BootstrapEnvVar names the environment variable that selects the bootstrap
// profile when --bootstrap is not given. MCP servers are configured per project
// in the client's own config, which makes an environment variable the natural
// place to pin a project's entry point.
const BootstrapEnvVar = "CONTOUR_BOOTSTRAP"

var mcpBootstrap string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the contour MCP server over stdio",
	Long: "Serve the contour store to an MCP client over stdio.\n\n" +
		"The selected bootstrap profile's rules are delivered eagerly in the " +
		"server's instructions, while skills and knowledge are reachable " +
		"through the list, search and get tools for on-demand fetching.\n\n" +
		"Select the profile with --bootstrap or the " + BootstrapEnvVar +
		" environment variable. Without one the server still serves the whole " +
		"store through its tools, and says how to choose an entry point.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve the store directly rather than through resolveStore: stdout
		// carries the MCP protocol, so nothing else may be written there.
		// Returning an error keeps the message on stderr.
		home, err := config.Resolve()
		if err != nil {
			return err
		}
		if !home.Exists {
			return fmt.Errorf("no contour store at %s (run `%s init`, or point %s at your store)",
				home.Path, config.Program, config.EnvVar)
		}

		profile := mcpBootstrap
		if profile == "" {
			profile = os.Getenv(BootstrapEnvVar)
		}

		if config.Dev {
			fmt.Fprintf(os.Stderr, "[%s build] contour mcp: store=%s profile=%q\n",
				config.Label, home.Path, profile)
		}

		// Shut down cleanly when the host stops us.
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return mcpserver.Run(ctx, mcpserver.Options{
			Root:    home.Path,
			Profile: profile,
			Version: cliVersion(),
		})
	},
}

func init() {
	mcpCmd.Flags().StringVar(&mcpBootstrap, "bootstrap", "",
		"bootstrap profile to load eagerly (falls back to "+BootstrapEnvVar+")")
	rootCmd.AddCommand(mcpCmd)
}

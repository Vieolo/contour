package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/unic"
	"github.com/vieolo/termange"
)

// mcpProfiles are the bootstrap profiles the running MCP server was started
// with. The bootstrap tool serves this session's payload rather than taking
// profile names as arguments: the client already chose the entry point, and
// letting the agent pick a different one would silently swap its own rules.
var mcpProfiles []string

type bootstrapInput struct{}

var bootstrapCmd = unic.UniversalCommand[bootstrapInput, any]{
	Use:   "bootstrap [name...]",
	Short: "Print the session-initialisation payload for one or more bootstrap profiles",
	Long: "Resolve one or more bootstrap profiles and print everything an agent " +
		"needs to start a session: the profile preambles, the rules they select " +
		"in full, and a menu of the skills and knowledge available to fetch on " +
		"demand.\n\n" +
		"Name several profiles to combine entry points — `bootstrap python cli` " +
		"for a Python project that also ships a CLI. They compose in the order " +
		"given, and an item selected by more than one is loaded once.\n\n" +
		"Run without a name to list the available profiles.\n\n" +
		"The payload is plain markdown on stdout — diagnostics go to stderr — " +
		"so it can be piped straight into an agent.",
	Description: "Load the complete set of rules in effect for this session, plus the " +
		"catalogue of skills and knowledge available on demand. The server's instructions " +
		"may carry only an excerpt, because the client limits how much they can hold — " +
		"this returns the whole thing.\n\n" +
		"If those instructions say they are incomplete, call this FIRST: before reading " +
		"a file, searching, planning or replying, and before any other tool. Not merely " +
		"before some later significant step — the rules you have not seen may govern the " +
		"very next thing you do, so there is no safe point at which to defer it.",
	Args: cobra.ArbitraryArgs,

	CLICommand: func(cmd *cobra.Command, args []string) error {
		home, err := resolveStore()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return listProfiles(home.Path)
		}

		profiles, err := bootstrap.LoadNamed(home.Path, args)
		if err != nil {
			return withAvailableProfiles(home.Path, err)
		}
		st, err := loadProjectStore(home.Path)
		if err != nil {
			return err
		}
		composed := bootstrap.Compose(profiles, st)

		// Diagnostics go to stderr so stdout stays a clean, pipeable payload.
		if config.Dev {
			fmt.Fprintf(os.Stderr, "[%s build] store: %s\n", config.Label, home.Path)
		}
		for _, u := range composed.UnmatchedTags {
			fmt.Fprintf(os.Stderr, "warning: profile %q requests tag %q, which matches no item\n", u.Profile, u.Tag)
		}

		fmt.Print(composed.Render(config.Program + " get <id>"))
		return nil
	},

	MCPCommand: func(ctx context.Context, req *mcp.CallToolRequest, in bootstrapInput) (*mcp.CallToolResult, any, error) {
		home, err := resolveStore()
		if err != nil {
			return nil, nil, asToolError(err)
		}
		st, err := loadProjectStore(home.Path)
		if err != nil {
			return nil, nil, asToolError(err)
		}
		profiles, err := bootstrap.LoadNamed(home.Path, mcpProfiles)
		if err != nil {
			return nil, nil, asToolError(err)
		}
		mcpUsage.Bootstrap()

		// Without a profile there are no eager central rules, but a project's own
		// local rules still apply — so this is never an empty answer by default.
		composed := bootstrap.Compose(profiles, st)
		return mcpserver.TextResult(composed.Render(mcpserver.FetchHint),
			"No rules are in effect: this store has no bootstrap profile selected and the project has no local rules."), nil, nil
	},
}

func init() {
	addCommand(&bootstrapCmd)
}

// listProfiles shows the available entry points. This branch is for a human, so
// unlike the payload it is colourised.
func listProfiles(root string) error {
	profiles, err := bootstrap.LoadProfiles(root)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		termange.PrintWarningf("No bootstrap profiles found in %s\n", bootstrap.Dir(root))
		termange.PrintInfof("Add one, then run: %s bootstrap <name>\n", config.Program)
		return nil
	}

	termange.PrintInfof("bootstrap profiles in %s\n\n", bootstrap.Dir(root))
	for _, p := range profiles {
		termange.PrintColorf(termange.ColorGreen, "  %s\n", p.Name)
		if p.Description != "" {
			termange.PrintInfof("      %s\n", p.Description)
		}
		termange.PrintColorf(termange.ColorYellow, "      rules: %s | skills: %s | knowledge: %s\n",
			tagList(p.RuleTags), tagList(p.SkillTags), tagList(p.KnowledgeTags))
	}
	termange.PrintInfof("\nEmit one with: %s bootstrap <name>\n", config.Program)
	termange.PrintInfof("Combine several: %s bootstrap <name> <name>\n", config.Program)
	return nil
}

// withAvailableProfiles enriches a lookup failure with the names that do exist.
func withAvailableProfiles(root string, cause error) error {
	profiles, err := bootstrap.LoadProfiles(root)
	if err != nil || len(profiles) == 0 {
		return cause
	}

	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return fmt.Errorf("%w (available: %s)", cause, strings.Join(names, ", "))
}

func tagList(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}
	return strings.Join(tags, ", ")
}

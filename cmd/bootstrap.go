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
	"github.com/vieolo/contour/internal/project"
	"github.com/vieolo/contour/internal/unic"
	"github.com/vieolo/termange"
)

// mcpProfiles are the bootstrap profiles the running MCP server was started
// with — the starting preset's, or --bootstrap's in their place.
var mcpProfiles []string

// mcpInInstructions holds the rules the running MCP server delivered in full in
// its initialisation instructions. Loading another preset leaves these out
// rather than repeat them: instructions stay in the agent's context for the
// whole session. Rules from earlier tool results are deliberately not counted —
// a client may compact those away, so "you already have it" could be false.
var mcpInInstructions = map[string]bool{}

type bootstrapInput struct {
	Preset string `json:"preset,omitempty" jsonschema:"a preset to load, by name, as listed in the server instructions; omit for this session's own rules"`
}

var bootstrapPreset string

var bootstrapCmd = unic.UniversalCommand[bootstrapInput, any]{
	Use:   "bootstrap [name...]",
	Short: "Print the session-initialisation payload for a preset or bootstrap profiles",
	Long: "Print everything an agent needs to start a session: the profile " +
		"preambles, the rules they select in full, and a menu of the skills and " +
		"knowledge available to fetch on demand.\n\n" +
		"With --preset, print the payload a session starting with that preset of " +
		"this project gets — its profiles and its eager files.\n\n" +
		"Or name profiles directly to see what they compose to — `bootstrap python " +
		"cli` for a Python project that also ships a CLI. They compose in the " +
		"order given, and an item selected by more than one is loaded once.\n\n" +
		"Run without either to list the available profiles, and this project's " +
		"presets.\n\n" +
		"The payload is plain markdown on stdout — diagnostics go to stderr — " +
		"so it can be piped straight into an agent.",
	Description: "Load rules and the catalogue of skills and knowledge that come with them.\n\n" +
		"Without `preset`, this returns the complete set of rules in effect for this session. " +
		"The server's instructions may carry only an excerpt, because the client limits how much " +
		"they can hold. If those instructions say they are incomplete, call this FIRST: before " +
		"reading a file, searching, planning or replying, and before any other tool. Not merely " +
		"before some later significant step — the rules you have not seen may govern the very " +
		"next thing you do, so there is no safe point at which to defer it.\n\n" +
		"With `preset`, this loads another of the project's presets — the ones listed in the " +
		"server instructions — when the user asks for it or the work is clearly about it. Its " +
		"rules apply from then on, alongside those already in effect.",
	Args: cobra.ArbitraryArgs,

	CLIFlags: func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&bootstrapPreset, "preset", "",
			"print the payload of this project's preset instead of naming profiles")
	},

	CLICommand: func(cmd *cobra.Command, args []string) error {
		home, err := resolveStore()
		if err != nil {
			return err
		}
		if bootstrapPreset != "" && len(args) > 0 {
			return fmt.Errorf("name a preset or profiles, not both")
		}

		overlays, cfg := projectContext()
		var (
			composed bootstrap.Composed
			missing  []string
		)
		switch {
		case bootstrapPreset != "":
			preset, ok := cfg.Preset(bootstrapPreset)
			if !ok {
				return fmt.Errorf("no preset named %q in this project%s", bootstrapPreset, presetsAvailable(cfg))
			}
			composed, missing, err = composePreset(home.Path, overlays, preset, preset.Bootstrap)
			if err != nil {
				return err
			}

		case len(args) == 0:
			return listProfiles(home.Path, cfg)

		default:
			// Named profiles are typed by a person, so a typo fails here. The
			// project's files come from its default preset, as a session's would.
			profiles, err := bootstrap.LoadNamed(home.Path, args)
			if err != nil {
				return withAvailableProfiles(home.Path, err)
			}
			def, _ := cfg.Select("")
			st, err := loadPresetStore(home.Path, overlays, def)
			if err != nil {
				return err
			}
			composed = bootstrap.Compose(profiles, st)
		}

		// Diagnostics go to stderr so stdout stays a clean, pipeable payload.
		if config.Dev {
			fmt.Fprintf(os.Stderr, "[%s build] store: %s\n", config.Label, home.Path)
		}
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "warning: profile(s) not in your store, left out: %s\n", strings.Join(missing, ", "))
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
		overlays, cfg := projectContext()

		name := strings.TrimSpace(in.Preset)
		if name == "" {
			// This session's own payload, whole — the path an incomplete set of
			// instructions sends the agent down.
			preset, _ := cfg.Select(mcpPreset)
			composed, missing, err := composePreset(home.Path, overlays, preset, mcpProfiles)
			if err != nil {
				return nil, nil, asToolError(err)
			}
			mcpUsage.Bootstrap(preset.Name)
			return mcpserver.TextResult(mcpserver.MissingProfilesNotice(missing)+composed.Render(mcpserver.FetchHint),
				"No rules are in effect: this session has no bootstrap profile and the project has no local rules."), nil, nil
		}

		preset, ok := cfg.Preset(name)
		if !ok {
			return nil, nil, mcpserver.NotFound(
				fmt.Sprintf("this project has no preset named %q", name),
				"pass one of the presets listed in the server instructions"+presetsAvailable(cfg))
		}
		composed, missing, err := composePreset(home.Path, overlays, preset, preset.Bootstrap)
		if err != nil {
			return nil, nil, asToolError(err)
		}
		mcpUsage.Bootstrap(preset.Name)

		body, repeated := composed.RenderSkipping(func(id string) bool { return mcpInInstructions[id] }, mcpserver.FetchHint)
		return mcpserver.TextResult(presetHeader(preset, repeated)+mcpserver.MissingProfilesNotice(missing)+body,
			fmt.Sprintf("The %q preset adds nothing beyond what is already in effect.", preset.Name)), nil, nil
	},
}

func init() {
	addCommand(&bootstrapCmd)
}

// composePreset composes a preset as a session starting with it would get it:
// the given profiles — normally the preset's own — over the store with that
// preset's eager files. A profile missing from this machine's store is reported,
// not fatal, so the rest still arrives.
func composePreset(root string, overlays []string, preset project.Preset, profileNames []string) (bootstrap.Composed, []string, error) {
	st, err := loadPresetStore(root, overlays, preset)
	if err != nil {
		return bootstrap.Composed{}, nil, err
	}
	profiles, missing, err := bootstrap.LoadAvailable(root, profileNames)
	if err != nil {
		return bootstrap.Composed{}, nil, err
	}
	return bootstrap.Compose(profiles, st), missing, nil
}

// presetHeader opens a preset loaded part-way through a session: what it is,
// that it adds to rather than replaces what is in effect, and which of its
// rules were left out because the agent already has them.
func presetHeader(preset project.Preset, repeated []string) string {
	var b strings.Builder
	if preset.Description != "" {
		fmt.Fprintf(&b, "# Preset: %s — %s\n\n", preset.Name, preset.Description)
	} else {
		fmt.Fprintf(&b, "# Preset: %s\n\n", preset.Name)
	}
	b.WriteString("These rules apply from now on, alongside those already in effect.")
	if len(repeated) > 0 {
		fmt.Fprintf(&b, " Also part of this preset, and already in effect since the session started, "+
			"so not repeated here: %s.", strings.Join(repeated, ", "))
	}
	b.WriteString("\n\n")
	return b.String()
}

// presetsAvailable renders " (available: a, b)" for an error message, or
// nothing when the project has no presets.
func presetsAvailable(cfg project.Config) string {
	if len(cfg.Presets) == 0 {
		return ""
	}
	return " (available: " + strings.Join(cfg.Names(), ", ") + ")"
}

// listProfiles shows the available entry points: the store's profiles, and this
// project's presets. This branch is for a human, so unlike the payload it is
// colourised.
func listProfiles(root string, cfg project.Config) error {
	if len(cfg.Presets) > 0 {
		termange.PrintInfof("presets in %s\n\n", cfg.Path)
		for _, p := range cfg.Presets {
			termange.PrintColorf(termange.ColorGreen, "  %s\n", p.Name)
			if p.Description != "" {
				termange.PrintInfof("      %s\n", p.Description)
			}
			termange.PrintColorf(termange.ColorYellow, "      bootstrap: %s\n", tagList(p.Bootstrap))
		}
		termange.PrintInfof("\nEmit one with: %s bootstrap --preset <name>\n\n", config.Program)
	}

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

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/mcpconfig"
	"github.com/vieolo/contour/internal/project"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/termange"
	"github.com/vieolo/termange/tui"
)

var (
	mcpInitProfiles    []string
	mcpInitInteractive bool
	mcpInitFile        string
	mcpInitName        string
)

// commonAgentFiles are the single-file conventions mcp-init offers to load
// eagerly, so a project that already keeps one need not restructure it.
var commonAgentFiles = []string{"AGENTS.md", "AGENT.md", "CLAUDE.md"}

var mcpInitCmd = &cobra.Command{
	Use:   "mcp-init",
	Short: "Register contour as an MCP server in this project",
	Long: "Register contour as an MCP server for this project on this machine, " +
		"and give the project a contour config if it has none.\n\n" +
		"The MCP entry (in " + mcpconfig.DefaultFile + ") is always written. It " +
		"records contour's absolute path — an agent launches its servers without " +
		"a login shell, so a bare `contour` would not resolve — which makes it " +
		"specific to this machine: keep " + mcpconfig.DefaultFile + " out of git, " +
		"and have each developer run this once.\n\n" +
		"The project config pins the profiles and lists any files to load " +
		"eagerly. It goes in the project's go.yaml, under external.contour, if it " +
		"has one — a Go project need not carry a second config file — and " +
		"otherwise in .contour.yaml at the project root. It is written only when " +
		"the project has none: an existing config is the team's committed setup, " +
		"and is never modified here. To change it, edit the file.\n\n" +
		"Pass --bootstrap to choose the profiles for a new config, repeating it to " +
		"combine entry points (--bootstrap python --bootstrap cli), or -i to pick " +
		"them from a list of the profiles in your store, with their descriptions. " +
		"Any AGENTS.md or CLAUDE.md at the project root is detected and listed for " +
		"eager loading. Against an existing config, --bootstrap is accepted only " +
		"if it names the profiles the config already selects, and -i is refused.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The path contour was launched from. Deliberately not resolved through
		// symlinks: Homebrew's bin entry points into a versioned Cellar
		// directory, and recording that would break on the next upgrade.
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determine contour's own path: %w", err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("determine the working directory: %w", err)
		}

		if mcpInitInteractive {
			picked, err := pickProfiles(cwd)
			if errors.Is(err, tui.ErrCanceled) {
				return nil // the picker has already said nothing was written
			}
			if err != nil {
				return err
			}
			mcpInitProfiles = picked
		}
		if err := checkProfilesExist(mcpInitProfiles); err != nil {
			return err
		}

		// 1. Give the project a config if it has none. An existing one is the
		//    team's committed setup and is left alone — every developer runs
		//    this command to register contour on their own machine, so it must
		//    not be able to rewrite shared settings. Settled before .mcp.json is
		//    touched, so a conflict leaves nothing half-done.
		cfg, created, err := project.Ensure(cwd, mcpInitProfiles, detectEagerFiles(cwd))
		if errors.Is(err, project.ErrProfileConflict) {
			def, _ := cfg.Select("")
			return fmt.Errorf("%s already exists (default preset: bootstrap [%s]) and mcp-init never changes an existing project config — "+
				"edit that file to switch profiles, or run without --bootstrap to just register contour on this machine",
				cfg.Path, strings.Join(def.Bootstrap, ", "))
		}
		if err != nil {
			return err
		}
		for _, w := range cfg.Warnings {
			fmt.Fprintf(os.Stderr, "contour: %s\n", w)
		}

		// 2. Register the server with a bare `mcp`; the profiles come from the
		//    project config, not the launch arguments.
		replaced, err := mcpconfig.Upsert(mcpInitFile, mcpInitName, mcpconfig.Entry{
			Command: exe,
			Args:    []string{"mcp"},
		})
		if err != nil {
			return err
		}

		if replaced {
			termange.PrintSuccessf("Updated the %q server in %s\n", mcpInitName, mcpInitFile)
		} else {
			termange.PrintSuccessf("Added the %q server to %s\n", mcpInitName, mcpInitFile)
		}
		termange.PrintInfof("  command: %s\n", exe)
		switch {
		case created:
			termange.PrintInfof("  config:  %s (created)\n", cfg.Path)
		case cfg.Migrated:
			termange.PrintInfof("  config:  %s (migrated to presets — review and commit it)\n", cfg.Path)
		default:
			termange.PrintInfof("  config:  %s (existing, left unchanged)\n", cfg.Path)
		}
		if len(cfg.Presets) > 1 {
			termange.PrintInfof("  presets: %s\n", strings.Join(cfg.Names(), ", "))
		}
		def, _ := cfg.Select("")
		for _, p := range def.Bootstrap {
			termange.PrintInfof("  profile: %s\n", p)
		}
		for _, f := range def.EagerFiles {
			termange.PrintInfof("  eager:   %s\n", f.ID)
		}

		if len(def.Bootstrap) == 0 {
			termange.PrintWarningf("\nThe default preset loads no profile — add `bootstrap: [<name>]` to it in %s (%s bootstrap lists them).\n",
				cfg.Path, config.Program)
		}
		if !created {
			warnMissingProfiles(cfg)
		}
		return nil
	},
}

func init() {
	mcpInitCmd.Flags().StringSliceVar(&mcpInitProfiles, "bootstrap", nil,
		"bootstrap profile for a new project config (repeatable)")
	mcpInitCmd.Flags().BoolVarP(&mcpInitInteractive, "interactive", "i", false,
		"pick the profiles for a new project config from a list (needs a terminal)")
	mcpInitCmd.MarkFlagsMutuallyExclusive("interactive", "bootstrap")
	mcpInitCmd.Flags().StringVar(&mcpInitFile, "file", mcpconfig.DefaultFile,
		"MCP config file to create or update")
	mcpInitCmd.Flags().StringVar(&mcpInitName, "name", config.Program,
		"name to register the server under")
	rootCmd.AddCommand(mcpInitCmd)
}

// detectEagerFiles finds common agent-instruction files at the project root, so
// mcp-init can list them for eager loading rather than making the user restate
// what they already keep.
func detectEagerFiles(projectDir string) []string {
	var found []string
	for _, name := range commonAgentFiles {
		if info, err := os.Stat(filepath.Join(projectDir, name)); err == nil && !info.IsDir() {
			found = append(found, name)
		}
	}
	return found
}

// pickProfiles lets the user choose a new project's profiles from the ones in
// their store, for when they remember what they want but not what it is called.
//
// Everything that would make the choice pointless is settled before the picker
// opens. An existing project config is refused outright, since mcp-init never
// changes one — better to say so up front than after the user has chosen. A
// missing terminal is refused too: the picker would otherwise block forever on
// input that never comes, which is how it behaves for an agent or a script.
func pickProfiles(projectDir string) ([]string, error) {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return nil, fmt.Errorf("--interactive needs a terminal; pass --bootstrap <name> instead (`%s bootstrap` lists the profiles)", config.Program)
	}

	existing, err := project.Load(projectDir)
	if err != nil {
		return nil, err
	}
	if existing.Path != "" {
		def, _ := existing.Select("")
		return nil, fmt.Errorf("%s already exists (default preset: bootstrap [%s]) and mcp-init never changes an existing project config — "+
			"run without --interactive to just register contour on this machine, or edit that file to switch profiles",
			existing.Path, strings.Join(def.Bootstrap, ", "))
	}

	home, err := resolveStore()
	if err != nil {
		return nil, err
	}
	profiles, err := bootstrap.LoadProfiles(profileSources(home.Path, store.DiscoverOverlays(projectDir)))
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("there are no bootstrap profiles in %s to choose from — add one first", bootstrap.Dir(home.Path))
	}

	items, nameOf := profileChoices(profiles)
	names := func(labels []string) []string {
		out := make([]string, 0, len(labels))
		for _, l := range labels {
			out = append(out, nameOf[l])
		}
		return out
	}

	labels, err := tui.MultiSelect(tui.MultiSelectOptions{
		Title: "Bootstrap profiles for this project",
		Items: items,
		SelectMessage: func(labels []string) string {
			if len(labels) == 0 {
				return "Nothing selected."
			}
			return "Selected: " + strings.Join(names(labels), ", ")
		},
		QuitMessage:     "Canceled — nothing was written.",
		EnableFiltering: true,
	})
	if err != nil {
		return nil, err
	}

	// Confirming with nothing selected is almost always a slip — pressing enter
	// on the highlighted profile expecting that to pick it, when it is space
	// that toggles. Writing an empty config would be costly to undo: mcp-init
	// will never change it afterwards, so the fix would be a hand edit.
	if len(labels) == 0 {
		return nil, errors.New("no profile selected (space selects, enter confirms) — nothing was written")
	}
	return names(labels), nil
}

// profileChoices turns profiles into picker items, returning the label each
// one is shown with and the mapping back to its name.
//
// The description goes into the label because the picker exists for people
// who have forgotten the names: what a profile is for is what they will
// recognise. It also makes the filter match descriptions as well as names.
// Profiles arrive sorted by name, and the picker returns its selection in the
// same order — which is therefore the order they compose in.
func profileChoices(profiles []bootstrap.Profile) ([]tui.SelectItem, map[string]string) {
	items := make([]tui.SelectItem, 0, len(profiles))
	nameOf := make(map[string]string, len(profiles))
	for _, p := range profiles {
		label := p.Name
		if p.Description != "" {
			label = p.Name + " — " + p.Description
		}
		items = append(items, tui.SelectItem(label))
		nameOf[label] = p.Name
	}
	return items, nameOf
}

// isTerminal reports whether f is attached to a terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// warnMissingProfiles tells a developer which of the profiles the project's
// presets use are missing from their store. The config came from the team, but
// the store is this machine's; saying so at setup beats discovering it from a
// session that quietly lacks some rules.
func warnMissingProfiles(cfg project.Config) {
	var names []string
	for _, p := range cfg.Presets {
		names = append(names, p.Bootstrap...)
	}
	if len(names) == 0 {
		return
	}
	home, err := resolveStore()
	if err != nil {
		return
	}
	_, missing, err := bootstrap.LoadAvailable(profileSources(home.Path, cwdOverlays()), slices.Compact(sortedCopy(names)))
	if err != nil || len(missing) == 0 {
		return
	}
	termange.PrintWarningf("\nThis project's presets use profiles found in neither the project nor your store: %s\n"+
		"Sessions will start without their rules until you add them (%s bootstrap lists yours).\n",
		strings.Join(missing, ", "), config.Program)
}

func sortedCopy(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// checkProfilesExist rejects an unknown profile now rather than leaving the
// agent to start a session with nothing loaded.
func checkProfilesExist(names []string) error {
	if len(names) == 0 {
		return nil
	}
	home, err := resolveStore()
	if err != nil {
		return err
	}
	src := profileSources(home.Path, cwdOverlays())
	if _, err := bootstrap.LoadNamed(src, names); err != nil {
		return withAvailableProfiles(src, err)
	}
	return nil
}

// cwdOverlays returns the working directory's overlays — the project's, since
// mcp-init runs from its root.
func cwdOverlays() []string {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	return store.DiscoverOverlays(wd)
}

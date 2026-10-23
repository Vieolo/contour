// Package mcpserver builds and runs the MCP server that exposes a contour store
// to a client over stdio.
//
// It owns the server's lifetime and its initialisation instructions — the eager
// bootstrap payload — but not the tools. Those are defined alongside their CLI
// counterparts under cmd/ and attached by the caller, so a command's two
// surfaces live in one file.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/store"
)

// FetchHint tells an agent how to retrieve a menu item over MCP. The CLI passes
// its own hint to the same renderer.
const FetchHint = "the `get` tool with the item's ID"

// InstructionsBudget caps the initialisation instructions.
//
// The MCP specification describes `instructions` as a hint a client "may" add to
// the system prompt — not a delivery guarantee — and clients cap it, cutting
// from the end. A store whose eager rules outgrow that cap therefore loses its
// last rules silently, which is the worst possible failure: the agent cannot
// tell that anything is missing.
//
// So contour sends an excerpt it knows fits, and says what it withheld. The
// figure is deliberately conservative: the caps are undocumented and vary by
// client, so the budget is set below the smallest one observed rather than at it.
const InstructionsBudget = 1900

// noticeReserve is the room set aside for the excerpt notice and its closing
// marker. It is only deducted once the payload is known not to fit, so a store
// within budget keeps the whole of it.
//
// Every character here is one the rules do not get, and rules are taken whole,
// so over-reserving by even a little can cost an entire rule. It is therefore
// sized just above the widest the strings can actually render — 623 characters
// at implausible six-figure counts — rather than guessed generously.
// TestNoticeReserveCoversTheNotice keeps the two in step.
const noticeReserve = 640

// Options configures the server.
type Options struct {
	// Root is the contour store's directory.
	Root string

	// Overlays are the project-local directories layered over the store, whose
	// items are always active for this project.
	Overlays []string

	// EagerFiles are single files (e.g. AGENTS.md) loaded eagerly as local
	// rules, from the project config.
	EagerFiles []store.EagerFile

	// Profiles are the bootstrap profiles loaded eagerly, composed in order.
	// They are looked up in the project's overlays first, then the central
	// store. When empty no profile rules are preloaded, and the instructions
	// explain where profiles are chosen.
	Profiles []string

	// SelectedLocal offers only the project items the profiles select, plus
	// the eager files — a preset's `local: selected`. The zero value includes
	// every project item, the default, so a caller that never heard of the
	// setting cannot drop a project's rules by leaving it out.
	SelectedLocal bool

	// Notices are things the agent should know about how the session was set
	// up — a requested preset that does not exist, say. Each is a paragraph,
	// placed right after the opening lines, where a client that caps the
	// instructions cannot cut it.
	Notices []string

	// Preset is the preset the session started with, and Presets all of the
	// project's presets; the others are listed for the agent to load.
	Preset  string
	Presets []PresetSummary
}

// New builds the server with the given initialisation instructions — normally
// BuildInstructions' Text. The caller registers tools before serving.
func New(instructions, version string) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{
		Name:    "contour",
		Title:   "contour context provider",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: instructions,
	})
}

// Serve runs the server on stdio, blocking until the client disconnects or ctx
// is cancelled.
//
// A client closing the pipe, or a signal cancelling ctx, is an ordinary
// shutdown rather than a failure worth reporting to the user.
//
// Nothing may be written to stdout but protocol traffic; callers must send
// their diagnostics to stderr.
func Serve(ctx context.Context, s *mcp.Server) error {
	if err := s.Run(ctx, &mcp.StdioTransport{}); err != nil &&
		!errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// TextResult wraps text as tool output, substituting fallback when it is empty
// so the model never receives a blank response.
func TextResult(text, fallback string) *mcp.CallToolResult {
	text = strings.TrimSpace(text)
	if text == "" {
		text = fallback
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// Instructions are what the client receives at initialisation.
type Instructions struct {
	// Text is the instructions themselves.
	Text string

	// RuleIDs are the rules whose full bodies Text carries. They stay in the
	// agent's context for the whole session — unlike a tool result, which a
	// client may compact away — so loading a preset later need not repeat
	// them.
	RuleIDs []string
}

// BuildInstructions composes what the client receives at initialisation: a short
// preamble, any notices, the project's other presets, the session's eager rules,
// and menus of what can be fetched.
func BuildInstructions(opts Options) (Instructions, error) {
	st, err := store.LoadProject(opts.Root, opts.Overlays, opts.EagerFiles)
	if err != nil {
		return Instructions{}, err
	}

	var b strings.Builder
	b.WriteString(intro)
	for _, n := range opts.Notices {
		b.WriteString(strings.TrimSpace(n))
		b.WriteString("\n\n")
	}
	b.WriteString(presetsSection(opts.Preset, opts.Presets))

	sources := bootstrap.Sources{Central: opts.Root, Overlays: opts.Overlays}
	if len(opts.Profiles) == 0 {
		writeNoProfileNotice(&b, sources)
		return noProfileInstructions(b.String(), st, !opts.SelectedLocal), nil
	}

	profiles, missing, err := bootstrap.LoadAvailable(sources, opts.Profiles)
	if err != nil {
		return Instructions{}, err
	}
	b.WriteString(MissingProfilesNotice(missing))
	if len(profiles) == 0 {
		// Every profile the project asked for is missing. Serve the session as
		// if none had been selected — local rules eager, the whole store on the
		// menu — rather than an empty one.
		return noProfileInstructions(b.String(), st, !opts.SelectedLocal), nil
	}
	composed := bootstrap.Compose(profiles, st, !opts.SelectedLocal)

	// Try the whole payload first; only if it will not fit does the notice cost
	// anything, so a store within budget is delivered exactly as before.
	avail := InstructionsBudget - b.Len()
	ex := composed.RenderWithin(avail, FetchHint)
	if ex.Complete {
		b.WriteString(ex.Body)
		return Instructions{Text: b.String(), RuleIDs: ex.ShownIDs}, nil
	}

	ex = composed.RenderWithin(avail-noticeReserve, FetchHint)
	b.WriteString(excerptNotice(ex))
	b.WriteString(ex.Body)
	b.WriteString(excerptCloser(ex))
	return Instructions{Text: b.String(), RuleIDs: ex.ShownIDs}, nil
}

// PresetSummary is what the instructions say about one of the project's presets.
type PresetSummary struct {
	Name        string
	Description string
}

// presetsSection lists the project's presets other than the one the session
// started with, so the agent knows what it can load and when. It renders as the
// empty string when there are none.
//
// It sits near the top, with the notices, rather than with the menus at the end:
// in a project set up around a light default, these are the main way into the
// rest of its context, and the menus are the first thing a long payload drops.
func presetsSection(current string, presets []PresetSummary) string {
	var others []PresetSummary
	for _, p := range presets {
		if p.Name != current {
			others = append(others, p)
		}
	}
	if len(others) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("# Presets\n\n")
	fmt.Fprintf(&b, "This session started with the %q preset. The project defines others, each a starting point for "+
		"one part of it. Load one with the `bootstrap` tool, passing its name as `preset`, when the user asks for it "+
		"or the work is clearly about it:\n\n", current)
	for _, p := range others {
		if p.Description != "" {
			fmt.Fprintf(&b, "- %s — %s\n", p.Name, p.Description)
		} else {
			fmt.Fprintf(&b, "- %s\n", p.Name)
		}
	}
	b.WriteString("\n")
	return b.String()
}

// noProfileInstructions builds the instructions for a session with no profile.
// The project's rules still load eagerly — all of them, or with allLocal off,
// just its eager files — so this path carries real eager content and is
// budgeted like any other: a project adopting contour with local rules alone
// must not lose them silently either.
//
// Menus go first when space runs short: the list tool reproduces them on demand,
// whereas a local rule body has no other eager route to the agent.
func noProfileInstructions(head string, st *store.Store, allLocal bool) Instructions {
	// The project's rules are eager — all of them, or under local: selected
	// only the files the config names, since with no profile nothing else is
	// selected.
	var localRules []store.Item
	for _, it := range st.BySource(store.KindRules, store.OriginLocal) {
		if allLocal || it.Pinned {
			localRules = append(localRules, it)
		}
	}

	// Central rules aren't eager here, but they remain fetchable, so they join
	// the menu alongside the on-demand skills and knowledge. Under local:
	// selected the project's unselected items stay off the menus too: a menu
	// line is startup context as much as a rule is, and keeping a large local
	// store out of it is what the setting is for. They stay reachable through
	// list, search and get.
	menuItems := func(kind store.Kind) []store.Item {
		if allLocal {
			return st.ByKind(kind)
		}
		return st.BySource(kind, store.OriginStore)
	}
	menus := bootstrap.RenderMenu("Available rules", st.BySource(store.KindRules, store.OriginStore), FetchHint) +
		bootstrap.RenderMenu("Available skills", menuItems(store.KindSkills), FetchHint) +
		bootstrap.RenderMenu("Available knowledge", menuItems(store.KindKnowledge), FetchHint)

	// Only the local rules are eager, so they are the whole budgeted payload.
	local := bootstrap.Composed{Rules: localRules}
	if full := local.RenderWithin(0, FetchHint); len(head)+len(full.Body)+len(menus)+1 <= InstructionsBudget {
		return Instructions{Text: strings.TrimRight(head+joinBlocks(full.Body, menus), "\n") + "\n", RuleIDs: full.ShownIDs}
	}

	// Menus dropped. If the local rules alone still overrun, excerpt them and say so.
	avail := InstructionsBudget - len(head)
	if ex := local.RenderWithin(avail, FetchHint); ex.Complete {
		return Instructions{
			Text: strings.TrimRight(head+ex.Body, "\n") + "\n" +
				"\nThe skills and knowledge menus were omitted for length — use the `list` tool to see them.\n",
			RuleIDs: ex.ShownIDs,
		}
	}
	ex := local.RenderWithin(avail-noticeReserve, FetchHint)
	return Instructions{
		Text:    strings.TrimRight(head+excerptNotice(ex)+ex.Body+excerptCloser(ex), "\n") + "\n",
		RuleIDs: ex.ShownIDs,
	}
}

// joinBlocks joins two markdown blocks with a blank line between them. A
// rendered payload ends in a single newline, so concatenating the next section
// straight on would run its heading into the last line of text.
func joinBlocks(a, b string) string {
	if a == "" || b == "" {
		return a + b
	}
	return strings.TrimRight(a, "\n") + "\n\n" + b
}

// MissingProfilesNotice tells the agent which of the project's profiles are not
// in this machine's store, and so were not loaded. It renders as the empty
// string when none are missing.
//
// The agent is asked to pass it on because the developer is the only one who
// can fix it, and nothing else would tell them: the session otherwise looks
// healthy, just quietly missing some of the rules the project relies on. It
// goes near the top of the instructions, where a client that caps them cannot
// cut it.
func MissingProfilesNotice(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("NOTE: this project uses the bootstrap %s %s, which %s in neither the project nor this "+
		"machine's contour store, so %s rules are not loaded. Mention this to the user so they can add %s.\n\n",
		plural(len(missing), "profile", "profiles"), strings.Join(missing, ", "),
		plural(len(missing), "is", "are"), plural(len(missing), "its", "their"),
		plural(len(missing), "it", "them"))
}

// intro opens the instructions. It states that the rules are binding before any
// of them appear, because a client that caps this field cuts from the end: what
// is written first is what survives.
const intro = "contour provides the centralised rules, skills and knowledge for this session.\n" +
	"The rules below are already in effect — follow them eagerly, without being asked.\n" +
	"Use the `list`, `search` and `get` tools to pull anything else on demand.\n\n"

// excerptNotice tells the agent that what follows is partial, how partial, and
// what to do about it.
//
// The sizes are stated deliberately. An agent told only "more rules exist" has
// no way to judge whether fetching them is worth a tool call; an agent told it
// is holding 1,158 of 6,055 characters knows it is missing most of its
// instructions. It goes above the rules for the same reason as the intro — a
// directive that trails the content it describes is the first thing a capping
// client discards.
//
// The wording is the product of watching it fail. "Call bootstrap before your
// first action" was read as "before the first *important* action", and deferred
// indefinitely: "action" is elastic, and an excerpt that looks like a usable
// rule set makes postponing feel safe. So the directive now names a specific
// event that cannot be reinterpreted — the first tool call — rejects the wrong
// reading in so many words, and says why deferring is not a judgement the agent
// is in a position to make: it cannot know which actions the unseen rules
// govern.
func excerptNotice(ex bootstrap.Excerpt) string {
	var b strings.Builder
	b.WriteString("## INCOMPLETE — `bootstrap` must be your FIRST tool call\n\n")

	missing := ex.TotalRules - ex.ShownRules
	fmt.Fprintf(&b, "Shown: %d of %d characters, %d of %d rules. The other %d %s in force and you have not been told what they say.\n",
		ex.ShownChars, ex.TotalChars, ex.ShownRules, ex.TotalRules,
		missing, plural(missing, "rule is", "rules are"))
	if !ex.MenusIncluded {
		b.WriteString("The skills and knowledge menus are omitted entirely.\n")
	}

	b.WriteString("\nCall `bootstrap` now — literally your next call, before reading a file, searching, planning or replying.\n")
	b.WriteString("\"Before the first important action\" is the wrong reading: you cannot know which actions the missing rules govern.\n")
	b.WriteString("What follows is a partial sample, not your instructions.\n\n")
	return b.String()
}

// excerptCloser repeats the directive where the excerpt ends. The notice at the
// top is read before the rules; by the end of them an agent has accumulated
// plausible-looking guidance and may have stopped treating the gap as real.
// Bracketing the sample keeps its incompleteness visible at the point it would
// otherwise be forgotten.
func excerptCloser(ex bootstrap.Excerpt) string {
	missing := ex.TotalRules - ex.ShownRules
	return fmt.Sprintf("\n— end of sample: %d of %d %s withheld. Call `bootstrap` before doing anything else. —\n",
		missing, ex.TotalRules, plural(missing, "rule", "rules"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// writeNoProfileNotice explains where profiles are chosen when the session has
// none, naming the profiles that exist.
func writeNoProfileNotice(b *strings.Builder, src bootstrap.Sources) {
	b.WriteString("No bootstrap profile is selected, so no profile rules were loaded eagerly.\n")
	b.WriteString("Profiles are chosen by the presets in the project's contour config.\n")

	profiles, err := bootstrap.LoadProfiles(src)
	if err != nil || len(profiles) == 0 {
		b.WriteString("\n")
		return
	}

	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	fmt.Fprintf(b, "Available profiles: %s\n\n", strings.Join(names, ", "))
}

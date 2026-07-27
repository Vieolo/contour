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
	// When empty no central rules are preloaded (local overlay rules still are),
	// and the instructions explain how to select an entry point.
	Profiles []string

	// Version is reported to clients during initialisation.
	Version string
}

// New builds the server with the bootstrap payload as its initialisation
// instructions. The caller registers tools before serving.
func New(opts Options) (*mcp.Server, error) {
	instructions, err := BuildInstructions(opts)
	if err != nil {
		return nil, err
	}

	return mcp.NewServer(&mcp.Implementation{
		Name:    "contour",
		Title:   "contour context provider",
		Version: opts.Version,
	}, &mcp.ServerOptions{
		Instructions: instructions,
	}), nil
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

// BuildInstructions composes what the client receives at initialisation: a short
// preamble, the profile's eager rules, and menus of what can be fetched.
func BuildInstructions(opts Options) (string, error) {
	st, err := store.LoadProject(opts.Root, opts.Overlays, opts.EagerFiles)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(intro)

	if len(opts.Profiles) == 0 {
		writeNoProfileNotice(&b, opts.Root)
		return noProfileInstructions(b.String(), st), nil
	}

	profiles, err := bootstrap.LoadNamed(opts.Root, opts.Profiles)
	if err != nil {
		return "", err
	}
	composed := bootstrap.Compose(profiles, st)

	// Try the whole payload first; only if it will not fit does the notice cost
	// anything, so a store within budget is delivered exactly as before.
	avail := InstructionsBudget - len(intro)
	ex := composed.RenderWithin(avail, FetchHint)
	if ex.Complete {
		b.WriteString(ex.Body)
		return b.String(), nil
	}

	ex = composed.RenderWithin(avail-noticeReserve, FetchHint)
	b.WriteString(excerptNotice(ex))
	b.WriteString(ex.Body)
	b.WriteString(excerptCloser(ex))
	return b.String(), nil
}

// noProfileInstructions builds the instructions for a session with no central
// profile. Local project rules still load eagerly, so this path carries real
// eager content and is budgeted like any other — a project adopting contour with
// local rules alone must not lose them silently either.
//
// Menus go first when space runs short: the list tool reproduces them on demand,
// whereas a local rule body has no other eager route to the agent.
func noProfileInstructions(head string, st *store.Store) string {
	localRules := st.BySource(store.KindRules, store.OriginLocal)
	// Central rules aren't eager here, but they remain fetchable, so list them
	// alongside the on-demand skills and knowledge.
	menus := bootstrap.RenderMenu("Available rules", st.BySource(store.KindRules, store.OriginStore), FetchHint) +
		bootstrap.RenderMenu("Available skills", st.ByKind(store.KindSkills), FetchHint) +
		bootstrap.RenderMenu("Available knowledge", st.ByKind(store.KindKnowledge), FetchHint)

	// Only the local rules are eager, so they are the whole budgeted payload.
	local := bootstrap.Composed{Rules: localRules}
	if full := local.RenderWithin(0, FetchHint); len(head)+len(full.Body)+len(menus) <= InstructionsBudget {
		return strings.TrimRight(head+full.Body+menus, "\n") + "\n"
	}

	// Menus dropped. If the local rules alone still overrun, excerpt them and say so.
	avail := InstructionsBudget - len(head)
	if ex := local.RenderWithin(avail, FetchHint); ex.Complete {
		return strings.TrimRight(head+ex.Body, "\n") + "\n" +
			"\nThe skills and knowledge menus were omitted for length — use the `list` tool to see them.\n"
	}
	ex := local.RenderWithin(avail-noticeReserve, FetchHint)
	return strings.TrimRight(head+excerptNotice(ex)+ex.Body+excerptCloser(ex), "\n") + "\n"
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

// writeNoProfileNotice explains how to select an entry point when the server was
// started without one, naming the profiles that exist.
func writeNoProfileNotice(b *strings.Builder, root string) {
	b.WriteString("No bootstrap profile is selected, so no rules were loaded eagerly.\n")
	b.WriteString("Select one or more by passing --bootstrap <name> when starting the server.\n")

	profiles, err := bootstrap.LoadProfiles(root)
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

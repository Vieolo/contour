// Package render draws contour's terminal output: colourised, indented and
// meant to be read by a person.
//
// It is the human counterpart to bootstrap.RenderMenu, which renders the same
// items as plain, compact text for a model's context. Commands that expose both
// surfaces call one from each handler, which is why the two live apart.
package render

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/contour/internal/usage"
	"github.com/vieolo/termange"
)

// StoreHeader announces which store the output came from, with a marker in
// development builds so dev and production output are never confused.
func StoreHeader(root string) {
	if config.Dev {
		termange.PrintWarningln(config.Label + " build — using the dev store")
	}
	termange.PrintInfof("contour store: %s\n", root)
}

// KindSection prints one kind's items, or a placeholder when it has none.
func KindSection(kind store.Kind, items []store.Item) {
	fmt.Println()
	termange.PrintColorf(termange.ColorGreen, "%s (%d)\n", string(kind), len(items))
	if len(items) == 0 {
		termange.PrintInfoln("  (none)")
		return
	}
	for _, it := range items {
		item(it)
	}
}

// NoMatches reports an empty search result.
func NoMatches(query string) {
	fmt.Println()
	termange.PrintWarningf("No items match %q.\n", query)
}

// TruncateLine shortens s to at most maxRunes runes, marking any cut with an
// ellipsis so one long line never floods the output. It counts runes, not
// bytes, so it never splits a multi-byte character.
func TruncateLine(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

// SearchKindHeader prints the "kind (n)" header above a group of search hits.
func SearchKindHeader(kind store.Kind, n int) {
	fmt.Println()
	termange.PrintColorf(termange.ColorGreen, "%s (%d)\n", string(kind), n)
	if n == 0 {
		termange.PrintInfoln("  (none)")
	}
}

// SearchHit prints one search match: the item, then either a bounded sample of
// the matching body lines with their numbers, or — for a metadata-only match —
// where the query was found instead.
func SearchHit(m store.Match, maxLines, maxLineLen int) {
	termange.PrintInfof("  %s\n", m.Item.ID)
	if m.Item.Description != "" {
		termange.PrintInfof("      %s\n", m.Item.Description)
	}

	if len(m.Lines) == 0 {
		termange.PrintColorf(termange.ColorYellow, "      matched in: %s\n", strings.Join(m.MatchedIn(), ", "))
		return
	}

	termange.PrintColorf(termange.ColorYellow, "      %d %s in body:\n",
		m.Occurrences, pluralize(m.Occurrences, "match", "matches"))
	shown := 0
	for _, ln := range m.Lines {
		if shown >= maxLines {
			break
		}
		termange.PrintInfof("      %d: %s\n", ln.Number, TruncateLine(ln.Text, maxLineLen))
		shown++
	}
	if extra := len(m.Lines) - shown; extra > 0 {
		termange.PrintInfof("      … %d more matching %s\n", extra, pluralize(extra, "line", "lines"))
	}
}

// SearchSummary prints the closing tally of a search.
func SearchSummary(occurrences, files int, query string) {
	fmt.Println()
	termange.PrintInfof("%d %s of %q across %d %s.\n",
		occurrences, pluralize(occurrences, "occurrence", "occurrences"),
		query, files, pluralize(files, "item", "items"))
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// Display caps for the usage report: the lists are review aids, not exhaustive
// dumps.
const (
	usageTopGaps    = 10
	usageTopFetched = 10
	usageMaxNever   = 20
)

// UsageReport prints the usage summary: gaps first (the most actionable), then
// never-fetched review candidates, then the most-fetched items, and any broken
// references. scope describes the filter in force (e.g. "all projects, all
// time").
func UsageReport(scope string, r *usage.Report, neverFetched []store.Item) {
	termange.PrintInfof("contour usage — %d %s across %d %s  (%s)\n",
		r.Sessions, pluralize(r.Sessions, "session", "sessions"),
		r.Projects, pluralize(r.Projects, "project", "projects"), scope)

	if r.Sessions == 0 {
		return
	}

	usageHeader("gaps — agents searched, found nothing")
	if len(r.Gaps) == 0 {
		usageNone()
	} else {
		for _, t := range capTallies(r.Gaps, usageTopGaps) {
			termange.PrintInfof("  %-30s %d×   last %s\n", `"`+t.Key+`"`, t.Count, ago(t.LastSeen))
		}
		usageMore(len(r.Gaps) - usageTopGaps)
	}

	usageHeader("never fetched — review or prune")
	if len(neverFetched) == 0 {
		usageNone()
	} else {
		shown := neverFetched
		if len(shown) > usageMaxNever {
			shown = shown[:usageMaxNever]
		}
		for _, it := range shown {
			termange.PrintInfof("  %s\n", it.ID)
		}
		usageMore(len(neverFetched) - len(shown))
	}

	usageHeader("most fetched")
	if len(r.Fetches) == 0 {
		usageNone()
	} else {
		for _, t := range capTallies(r.Fetches, usageTopFetched) {
			termange.PrintInfof("  %-42s %d×   last %s\n", t.Key, t.Count, ago(t.LastSeen))
		}
	}

	if len(r.MissingFetches) > 0 {
		usageHeader("fetched but missing — broken references")
		for _, t := range r.MissingFetches {
			termange.PrintInfof("  %-42s %d×\n", t.Key, t.Count)
		}
	}
}

func usageHeader(title string) {
	fmt.Println()
	termange.PrintColorf(termange.ColorGreen, "%s\n", title)
}

func usageNone() { termange.PrintInfoln("  (none)") }

func usageMore(extra int) {
	if extra > 0 {
		termange.PrintInfof("  … %d more\n", extra)
	}
}

func capTallies(t []usage.Tally, n int) []usage.Tally {
	if len(t) > n {
		return t[:n]
	}
	return t
}

// ago renders a coarse "how long ago" for a timestamp.
func ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	switch d := time.Since(t); {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// ProfilesHeader names the profiles a listing is cross-referenced against.
func ProfilesHeader(names []string) {
	if len(names) == 0 {
		termange.PrintWarningln("No bootstrap profiles, so nothing is offered at session start.")
		return
	}
	termange.PrintInfof("%d %s: %s\n",
		len(names), pluralize(len(names), "profile", "profiles"), strings.Join(names, ", "))
}

// KindSectionProfiles prints one kind's items annotated with the profiles that
// offer each. It builds on the same item renderer as KindSection, so a listing
// reads the same with the annotation as without it.
func KindSectionProfiles(kind store.Kind, items []bootstrap.ItemProfiles) {
	fmt.Println()
	termange.PrintColorf(termange.ColorGreen, "%s (%d)\n", string(kind), len(items))
	if len(items) == 0 {
		termange.PrintInfoln("  (none)")
		return
	}
	for _, ip := range items {
		item(ip.Item)
		profilesLine(ip)
	}
}

// profilesLine renders what offers an item, aligned with the tags line above it.
func profilesLine(ip bootstrap.ItemProfiles) {
	switch {
	case ip.AlwaysActive():
		termange.PrintColorf(termange.ColorGreen, "      profiles: always active (local)\n")
	case len(ip.Profiles) > 0:
		termange.PrintColorf(termange.ColorGreen, "      profiles: %s\n", strings.Join(ip.Profiles, ", "))
	default:
		termange.PrintColorf(termange.ColorYellow, "      profiles: none — not offered at session start\n")
	}
}

// UnofferedSummary closes a --profiles listing with what to do about the n items
// nothing offers — and, just as importantly, what not to conclude about them.
//
// The note matters as much as the count. An item no profile selects is still
// served by list, search and get, so calling it unreachable would be wrong; what
// it lacks is a profile putting it in front of the agent up front.
func UnofferedSummary(n int) {
	fmt.Println()
	if n == 0 {
		termange.PrintSuccessln("Every item is offered by at least one profile.")
		return
	}

	termange.PrintWarningf("%d %s in no profile.\n", n, pluralize(n, "item is", "items are"))
	termange.PrintInfoln("  Such items stay reachable — an agent can find them with list, search and get.")
	termange.PrintInfoln("  But nothing offers them when a session starts, so they are only used if the")
	termange.PrintInfoln("  agent goes looking. Give one a tag a profile selects to have it offered up front.")
}

// StoreCreated reports that contour set up the store, and explains how its
// layout works. First run is the one moment the user is guaranteed to see this,
// so it teaches the structure rather than just naming the path.
//
// Unlike the rest of this package it writes to stderr: commands such as get and
// bootstrap emit a payload on stdout that a banner would corrupt.
func StoreCreated(path, configFile string) {
	rule := termange.PaintText(strings.Repeat("─", 64), termange.ColorYellow)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n\n", rule,
		termange.PaintText("  contour — store created", termange.ColorYellow), rule)
	fmt.Fprintf(&b, "No store existed yet, so contour created one for you at:\n\n    %s\n\n",
		termange.PaintText(path, termange.ColorGreen))
	b.WriteString(storeLayoutHelp)
	fmt.Fprintf(&b, "\n%s\n    %s set-home /path/to/store\n\n",
		termange.PaintText("  To keep the store somewhere else", termange.ColorGreen), config.Program)
	fmt.Fprintf(&b, "%s\n    %s\n    Every setting contour has, with each one explained in the file.\n\n",
		termange.PaintText("  Configuration", termange.ColorGreen), configFile)
	fmt.Fprintf(&b, "%s\n", rule)

	fmt.Fprint(os.Stderr, b.String())
}

// ConfigCreated notes, on stderr, that contour wrote its config file. It covers
// the case where the store already existed and only the config was missing — the
// user would otherwise have no idea the file is there to edit.
func ConfigCreated(path string) {
	fmt.Fprintf(os.Stderr, "%s %s\n",
		termange.PaintText("contour: created its config file at", termange.ColorYellow), path)
}

const storeLayoutHelp = `Layout:

    bootstrap/   Named entry points. Each profile selects, by tag, which
                 rules load eagerly and which skills and knowledge are
                 offered on demand.
    rules/       Imperative "how to behave" guidance. Loaded eagerly.
    skills/      Procedural "how to do X". Fetched on demand.
    knowledge/   Reference facts. Fetched on demand.

Folder names under rules/, skills/ and knowledge/ become implicit tags, so a
file at rules/python/errors.md is tagged "python". Sample files are included;
edit or delete them freely, and see README.md in the store for the conventions.
`

// item renders a single item: ID, description and tags, marking local overlay
// items so their origin is visible.
func item(it store.Item) {
	suffix := ""
	if it.Source == store.OriginLocal {
		suffix = "  (local)"
	}
	termange.PrintInfof("  %s%s\n", it.ID, suffix)
	if it.Description != "" {
		termange.PrintInfof("      %s\n", it.Description)
	}
	if len(it.Tags) > 0 {
		termange.PrintColorf(termange.ColorYellow, "      tags: %s\n", strings.Join(it.Tags, ", "))
	}
}

package bootstrap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vieolo/contour/internal/store"
)

// localRulesHeading introduces the eager rules that come from a project overlay.
// It states the precedence policy to the agent rather than trying to detect
// conflicts: contour cannot know which local and central rules clash
// semantically, so it delegates reconciliation with a clear rule.
const (
	localRulesHeading = "# Project rules (local — authoritative on conflict)"
	localRulesNote    = "These come from this project and take precedence over the central store where they conflict."
)

// Composed is a profile resolved against a store: the rules to load eagerly, and
// the skills and knowledge offered for on-demand fetching. Each slice holds the
// central items the profile selected followed by every project-overlay item,
// which is always included regardless of tags.
type Composed struct {
	Profile   Profile
	Rules     []store.Item
	Skills    []store.Item
	Knowledge []store.Item

	// UnmatchedTags are tags the profile requested that matched no central item
	// in any kind they were requested for — almost always a typo. Overlay items
	// do not count, since they are included without tags.
	UnmatchedTags []string
}

// Compose resolves a profile's tag selections against the central store and adds
// every project-overlay item unconditionally.
func Compose(p Profile, st *store.Store) Composed {
	return Composed{
		Profile:       p,
		Rules:         composeKind(st, store.KindRules, p.RuleTags),
		Skills:        composeKind(st, store.KindSkills, p.SkillTags),
		Knowledge:     composeKind(st, store.KindKnowledge, p.KnowledgeTags),
		UnmatchedTags: unmatchedTags(st, p),
	}
}

// composeKind selects the central items of a kind by tag, then appends every
// local overlay item of that kind — project context applies unconditionally.
func composeKind(st *store.Store, kind store.Kind, tags []string) []store.Item {
	selected := selectByTags(st.BySource(kind, store.OriginStore), tags)
	return append(selected, st.BySource(kind, store.OriginLocal)...)
}

// selectByTags returns the items carrying any of the given tags. The tag list
// drives the order — every item matching the first tag, then any new items
// matching the second, and so on — never repeating an item.
func selectByTags(items []store.Item, tags []string) []store.Item {
	seen := make(map[string]bool)

	var out []store.Item
	for _, tag := range tags {
		for _, it := range items {
			if seen[it.ID] || !it.HasTag(tag) {
				continue
			}
			seen[it.ID] = true
			out = append(out, it)
		}
	}
	return out
}

// unmatchedTags reports the tags a profile requested that matched no central
// item in any of the kinds they were requested for.
func unmatchedTags(st *store.Store, p Profile) []string {
	requested := make(map[string][]store.Kind)
	for _, sel := range []struct {
		tags []string
		kind store.Kind
	}{
		{p.RuleTags, store.KindRules},
		{p.SkillTags, store.KindSkills},
		{p.KnowledgeTags, store.KindKnowledge},
	} {
		for _, tag := range sel.tags {
			requested[tag] = append(requested[tag], sel.kind)
		}
	}

	var out []string
	for tag, kinds := range requested {
		if !anyStoreItemHasTag(st, kinds, tag) {
			out = append(out, tag)
		}
	}
	sort.Strings(out) // map iteration is unordered; keep output deterministic
	return out
}

func anyStoreItemHasTag(st *store.Store, kinds []store.Kind, tag string) bool {
	for _, k := range kinds {
		for _, it := range st.BySource(k, store.OriginStore) {
			if it.HasTag(tag) {
				return true
			}
		}
	}
	return false
}

// Render returns the session-initialisation payload as markdown: the profile
// preamble, the selected central rules, the project's local rules (with the
// precedence note), and menus of the skills and knowledge available on demand.
//
// fetchHint describes how to retrieve a menu item (for example
// "contour get <id>") and is omitted when empty. Both the CLI and the MCP server
// render through here so the two surfaces stay consistent.
func (c Composed) Render(fetchHint string) string {
	var b strings.Builder

	if c.Profile.Preamble != "" {
		b.WriteString(c.Profile.Preamble)
		b.WriteString("\n\n")
	}

	storeRules, localRules := partitionBySource(c.Rules)
	if len(storeRules) > 0 {
		b.WriteString("# Rules\n\n")
		writeRules(&b, storeRules)
	}
	b.WriteString(RenderLocalRules(localRules))

	writeMenu(&b, "Available skills", c.Skills, fetchHint)
	writeMenu(&b, "Available knowledge", c.Knowledge, fetchHint)

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// RenderLocalRules renders the eager "project rules" section for a set of local
// overlay rules, stating that they are authoritative on conflict. Empty input
// renders as the empty string. It is shared by Compose.Render and the
// no-profile path so local rules always appear the same way.
func RenderLocalRules(rules []store.Item) string {
	if len(rules) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(localRulesHeading)
	b.WriteString("\n\n")
	b.WriteString(localRulesNote)
	b.WriteString("\n\n")
	writeRules(&b, rules)
	return b.String()
}

func writeRules(b *strings.Builder, rules []store.Item) {
	for _, it := range rules {
		fmt.Fprintf(b, "## %s\n\n", it.ID)
		if it.Body != "" {
			b.WriteString(it.Body)
			b.WriteString("\n\n")
		}
	}
}

func partitionBySource(items []store.Item) (central, local []store.Item) {
	for _, it := range items {
		if it.Source == store.OriginLocal {
			local = append(local, it)
		} else {
			central = append(central, it)
		}
	}
	return central, local
}

// writeMenu appends a progressive-disclosure menu: one line per item, carrying
// just enough (ID and description) for an agent to decide whether to fetch it.
// Local items are marked so their origin is never ambiguous.
func writeMenu(b *strings.Builder, title string, items []store.Item, fetchHint string) {
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(b, "# %s\n\n", title)
	if fetchHint != "" {
		fmt.Fprintf(b, "Fetch on demand with: %s\n\n", fetchHint)
	}
	for _, it := range items {
		local := ""
		if it.Source == store.OriginLocal {
			local = "  (local)"
		}
		if it.Description != "" {
			fmt.Fprintf(b, "- %s — %s%s\n", it.ID, it.Description, local)
		} else {
			fmt.Fprintf(b, "- %s%s\n", it.ID, local)
		}
	}
	b.WriteString("\n")
}

// RenderMenu renders a standalone progressive-disclosure menu for a set of
// items, in the same format Render uses. The MCP tools render through here so
// their output matches the bootstrap payload. An empty item list renders as the
// empty string.
func RenderMenu(title string, items []store.Item, fetchHint string) string {
	var b strings.Builder
	writeMenu(&b, title, items, fetchHint)
	return b.String()
}

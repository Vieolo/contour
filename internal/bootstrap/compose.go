package bootstrap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vieolo/contour/internal/store"
)

// Composed is a profile resolved against a store: the rules to load eagerly,
// and the skills and knowledge offered for on-demand fetching.
type Composed struct {
	Profile   Profile
	Rules     []store.Item
	Skills    []store.Item
	Knowledge []store.Item

	// UnmatchedTags are tags the profile requested that matched no item in any
	// kind they were requested for — almost always a typo.
	UnmatchedTags []string
}

// Compose resolves a profile's tag selections against the store.
func Compose(p Profile, st *store.Store) Composed {
	return Composed{
		Profile:       p,
		Rules:         selectByTags(st, store.KindRules, p.RuleTags),
		Skills:        selectByTags(st, store.KindSkills, p.SkillTags),
		Knowledge:     selectByTags(st, store.KindKnowledge, p.KnowledgeTags),
		UnmatchedTags: unmatchedTags(st, p),
	}
}

// selectByTags returns the items of a kind carrying any of the given tags. The
// tag list drives the order — every item matching the first tag, then any new
// items matching the second, and so on — using store order within each tag and
// never repeating an item.
func selectByTags(st *store.Store, kind store.Kind, tags []string) []store.Item {
	items := st.ByKind(kind)
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

// unmatchedTags reports the tags a profile requested that matched nothing in
// any of the kinds they were requested for.
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
		if !anyItemHasTag(st, kinds, tag) {
			out = append(out, tag)
		}
	}
	sort.Strings(out) // map iteration is unordered; keep output deterministic
	return out
}

func anyItemHasTag(st *store.Store, kinds []store.Kind, tag string) bool {
	for _, k := range kinds {
		for _, it := range st.ByKind(k) {
			if it.HasTag(tag) {
				return true
			}
		}
	}
	return false
}

// Render returns the session-initialisation payload as markdown: the profile
// preamble, the selected rules in full, and menus of the skills and knowledge
// available on demand.
//
// fetchHint describes how to retrieve a menu item (for example
// "contour get <id>") and is omitted when empty. Both the CLI and the MCP
// server render through here so the two surfaces stay consistent.
func (c Composed) Render(fetchHint string) string {
	var b strings.Builder

	if c.Profile.Preamble != "" {
		b.WriteString(c.Profile.Preamble)
		b.WriteString("\n\n")
	}

	if len(c.Rules) > 0 {
		b.WriteString("# Rules\n\n")
		for _, it := range c.Rules {
			fmt.Fprintf(&b, "## %s\n\n", it.ID)
			if it.Body != "" {
				b.WriteString(it.Body)
				b.WriteString("\n\n")
			}
		}
	}

	writeMenu(&b, "Available skills", c.Skills, fetchHint)
	writeMenu(&b, "Available knowledge", c.Knowledge, fetchHint)

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// writeMenu appends a progressive-disclosure menu: one line per item, carrying
// just enough (ID and description) for an agent to decide whether to fetch it.
func writeMenu(b *strings.Builder, title string, items []store.Item, fetchHint string) {
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(b, "# %s\n\n", title)
	if fetchHint != "" {
		fmt.Fprintf(b, "Fetch on demand with: %s\n\n", fetchHint)
	}
	for _, it := range items {
		if it.Description != "" {
			fmt.Fprintf(b, "- %s — %s\n", it.ID, it.Description)
		} else {
			fmt.Fprintf(b, "- %s\n", it.ID)
		}
	}
	b.WriteString("\n")
}

package bootstrap

import (
	"fmt"
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

// Composed is a set of profiles resolved against a store: the rules to load
// eagerly, and the skills and knowledge offered for on-demand fetching. Each
// slice holds the central items the profiles selected followed by every
// project-overlay item, which is always included regardless of tags.
type Composed struct {
	Profiles  []Profile
	Rules     []store.Item
	Skills    []store.Item
	Knowledge []store.Item

	// UnmatchedTags are tags a profile requested that matched no central item in
	// any kind it requested them for — almost always a typo. Overlay items do not
	// count, since they are included without tags.
	UnmatchedTags []UnmatchedTag
}

// UnmatchedTag is one such tag, paired with the profile that asked for it. The
// attribution is what makes the warning actionable once several profiles are
// active: it names the file to go and fix.
type UnmatchedTag struct {
	Profile string
	Tag     string
}

// Compose resolves the profiles' tag selections against the central store and
// adds every project-overlay item unconditionally.
//
// Profiles compose by concatenating their tag selections in the order given, so
// [python, cli] puts the Python rules ahead of the CLI ones, and an item both
// profiles select appears once, in the earlier position. That is exactly the
// rule that already orders tags within one profile, lifted a level — which is
// what lets a project assemble the entry point it needs (mostly Python, with an
// accessory CLI) without the store carrying a profile for every combination.
func Compose(profiles []Profile, st *store.Store) Composed {
	return Composed{
		Profiles:      profiles,
		Rules:         composeKind(st, store.KindRules, mergedTags(profiles, store.KindRules)),
		Skills:        composeKind(st, store.KindSkills, mergedTags(profiles, store.KindSkills)),
		Knowledge:     composeKind(st, store.KindKnowledge, mergedTags(profiles, store.KindKnowledge)),
		UnmatchedTags: unmatchedTags(st, profiles),
	}
}

// mergedTags concatenates the profiles' tag selections for a kind in profile
// order, dropping repeats so a tag two profiles share keeps its first position.
func mergedTags(profiles []Profile, kind store.Kind) []string {
	seen := make(map[string]bool)

	var out []string
	for _, p := range profiles {
		for _, tag := range p.TagsFor(kind) {
			if seen[tag] {
				continue
			}
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
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

// unmatchedTags reports, per profile, the tags it requested that matched no
// central item in any of the kinds it requested them for. Each profile is
// checked on its own so a typo is attributed to the file that contains it,
// rather than to whichever set of profiles happened to be selected together.
func unmatchedTags(st *store.Store, profiles []Profile) []UnmatchedTag {
	var out []UnmatchedTag
	for _, p := range profiles {
		requested := make(map[string][]store.Kind)
		var order []string // request order, so output is deterministic without sorting
		for _, kind := range store.Kinds {
			for _, tag := range p.TagsFor(kind) {
				if _, seen := requested[tag]; !seen {
					order = append(order, tag)
				}
				requested[tag] = append(requested[tag], kind)
			}
		}

		for _, tag := range order {
			if !anyStoreItemHasTag(st, requested[tag], tag) {
				out = append(out, UnmatchedTag{Profile: p.Name, Tag: tag})
			}
		}
	}
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
// preambles, the selected central rules, the project's local rules (with the
// precedence note), and menus of the skills and knowledge available on demand.
//
// fetchHint describes how to retrieve a menu item (for example
// "contour get <id>") and is omitted when empty. Both the CLI and the MCP server
// render through here so the two surfaces stay consistent.
func (c Composed) Render(fetchHint string) string {
	storeRules, localRules := partitionBySource(c.Rules)
	return c.render(storeRules, localRules, true, fetchHint)
}

// render builds the payload from an explicit choice of rules, so a budgeted
// excerpt and the complete payload are produced by the same code and cannot
// drift in wording or layout.
func (c Composed) render(storeRules, localRules []store.Item, withMenus bool, fetchHint string) string {
	var b strings.Builder

	// Each active profile's preamble, in profile order. They are separate
	// markdown blocks, so several read as consecutive paragraphs.
	for _, p := range c.Profiles {
		if p.Preamble != "" {
			b.WriteString(p.Preamble)
			b.WriteString("\n\n")
		}
	}

	if len(storeRules) > 0 {
		b.WriteString("# Rules\n\n")
		writeRules(&b, storeRules)
	}
	b.WriteString(RenderLocalRules(localRules))

	if withMenus {
		writeMenu(&b, "Available skills", c.Skills, fetchHint)
		writeMenu(&b, "Available knowledge", c.Knowledge, fetchHint)
	}

	// Nothing to say renders as nothing, not as a stray newline that would space
	// out whatever a caller concatenates around it.
	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		return ""
	}
	return out + "\n"
}

// Excerpt is a rendering of a composed profile trimmed to fit a character
// budget, together with what it had to leave out.
//
// It exists because the MCP `instructions` field is not a delivery guarantee.
// The specification calls it a hint that a client "may" add to the system
// prompt, and clients cap it — so a payload that outgrows the cap is cut from
// the end, silently. Sending a known-good excerpt and stating what is missing is
// strictly better than sending everything and hoping.
type Excerpt struct {
	// Body is the rendered payload, at most the requested budget unless even the
	// minimum content exceeds it.
	Body string

	// Complete reports whether Body is the whole payload. When true, every other
	// field describes the full content and no notice is warranted.
	Complete bool

	// ShownRules and TotalRules count the rules included versus selected.
	ShownRules, TotalRules int

	// ShownChars and TotalChars are the size of Body versus the complete payload
	// as rendered for this surface. They are close to, but not identical with,
	// what `contour bootstrap` prints: the CLI passes a different fetch hint. The
	// figures exist to convey magnitude to an agent, not to be reconciled
	// byte-for-byte against another command's output.
	ShownChars, TotalChars int

	// MenusIncluded reports whether the skills and knowledge menus survived. They
	// are dropped first: the list tool reproduces them on demand, while a rule
	// body has no other eager route to the agent.
	MenusIncluded bool
}

// RenderWithin renders the payload trimmed to fit budget characters.
//
// Rules are included whole or not at all — half a rule is worse than a missing
// one, since a body cut mid-sentence can invert its own meaning. Project-local
// rules get first claim on the budget: they are authoritative on conflict, so
// dropping them in favour of central rules they are meant to override would
// deliver a session that is not merely incomplete but wrong.
func (c Composed) RenderWithin(budget int, fetchHint string) Excerpt {
	full := c.Render(fetchHint)
	ex := Excerpt{
		Body:          full,
		Complete:      true,
		ShownRules:    len(c.Rules),
		TotalRules:    len(c.Rules),
		ShownChars:    len(full),
		TotalChars:    len(full),
		MenusIncluded: true,
	}
	if budget <= 0 || len(full) <= budget {
		return ex
	}

	storeRules, localRules := partitionBySource(c.Rules)

	// Local rules first, dropping from the end only if they alone overrun.
	nLocal := len(localRules)
	for nLocal > 0 && len(c.render(nil, localRules[:nLocal], false, fetchHint)) > budget {
		nLocal--
	}
	// Then as many central rules as still fit, in profile order.
	nStore := 0
	for nStore < len(storeRules) &&
		len(c.render(storeRules[:nStore+1], localRules[:nLocal], false, fetchHint)) <= budget {
		nStore++
	}

	body := c.render(storeRules[:nStore], localRules[:nLocal], false, fetchHint)
	menus := false
	if withMenus := c.render(storeRules[:nStore], localRules[:nLocal], true, fetchHint); len(withMenus) <= budget {
		body, menus = withMenus, true
	}

	ex.Body = body
	ex.Complete = false
	ex.ShownRules = nStore + nLocal
	ex.ShownChars = len(body)
	ex.MenusIncluded = menus
	return ex
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

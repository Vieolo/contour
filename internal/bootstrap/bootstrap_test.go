package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vieolo/contour/internal/store"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testRoot builds a store whose "python" profile exercises tag ordering and
// de-duplication: 020-shared lives under python/ but is also tagged general, so
// matches both selected tags and must appear exactly once, in the general block.
func testRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write(t, root, "bootstrap/python.md",
		"---\ndescription: Python projects\nrules: [general, python]\nskills: [python]\nknowledge: [general]\n---\nPreamble text.")
	write(t, root, "bootstrap/empty.md",
		"---\ndescription: nothing matches\nrules: [missingtag]\n---\n")
	// Sorts after "python" by name, but before it by filename ('-' < '.').
	write(t, root, "bootstrap/python-frontend.md",
		"---\ndescription: Python plus frontend\nrules: [general, python]\n---\n")

	write(t, root, "rules/general/010-comm.md", "---\ndescription: comm\n---\nBe concise.")
	write(t, root, "rules/python/010-errors.md", "---\ndescription: errs\n---\nWrap errors.")
	write(t, root, "rules/python/020-shared.md", "---\ndescription: shared\ntags: [general]\n---\nShared rule.")
	write(t, root, "rules/js/010-style.md", "---\ndescription: style\n---\nUse prettier.")
	write(t, root, "skills/python/release/SKILL.md", "---\ndescription: release\n---\nSteps")
	write(t, root, "knowledge/general/stack.md", "---\ndescription: stack\n---\nGo + Postgres.")

	return root
}

func compose(t *testing.T, root string, names ...string) Composed {
	t.Helper()
	profiles, err := LoadNamed(root, names)
	if err != nil {
		t.Fatalf("LoadNamed(%v): %v", names, err)
	}
	st, err := store.Load(root)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	return Compose(profiles, st)
}

func TestLoadProfile(t *testing.T) {
	root := testRoot(t)

	p, err := LoadProfile(root, "python")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if p.Name != "python" {
		t.Errorf("Name = %q, want python", p.Name)
	}
	if p.Description != "Python projects" {
		t.Errorf("Description = %q", p.Description)
	}
	if p.Preamble != "Preamble text." {
		t.Errorf("Preamble = %q", p.Preamble)
	}
	if want := []string{"general", "python"}; !reflect.DeepEqual(p.RuleTags, want) {
		t.Errorf("RuleTags = %v, want %v", p.RuleTags, want)
	}
	if want := []string{"python"}; !reflect.DeepEqual(p.SkillTags, want) {
		t.Errorf("SkillTags = %v, want %v", p.SkillTags, want)
	}
}

func TestLoadProfileMissing(t *testing.T) {
	if _, err := LoadProfile(testRoot(t), "nope"); err == nil {
		t.Error("LoadProfile returned nil error for a missing profile")
	}
}

func TestLoadProfiles(t *testing.T) {
	ps, err := LoadProfiles(testRoot(t))
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	if want := []string{"empty", "python", "python-frontend"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestLoadProfilesNoDir(t *testing.T) {
	ps, err := LoadProfiles(t.TempDir())
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if len(ps) != 0 {
		t.Errorf("got %d profiles, want 0", len(ps))
	}
}

func TestComposeOrderAndDedup(t *testing.T) {
	c := compose(t, testRoot(t), "python")

	var ids []string
	for _, it := range c.Rules {
		ids = append(ids, it.ID)
	}
	// general block first (010-comm, then 020-shared), then the python block adds
	// 010-errors; 020-shared is not repeated and js is never selected.
	want := []string{"rules/general/010-comm", "rules/python/020-shared", "rules/python/010-errors"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("rule order = %v, want %v", ids, want)
	}
}

func TestComposeLazyKinds(t *testing.T) {
	c := compose(t, testRoot(t), "python")

	if len(c.Skills) != 1 || c.Skills[0].ID != "skills/python/release" {
		t.Errorf("skills = %v, want [skills/python/release]", c.Skills)
	}
	if len(c.Knowledge) != 1 || c.Knowledge[0].ID != "knowledge/general/stack" {
		t.Errorf("knowledge = %v, want [knowledge/general/stack]", c.Knowledge)
	}
	if len(c.UnmatchedTags) != 0 {
		t.Errorf("UnmatchedTags = %v, want none", c.UnmatchedTags)
	}
}

func TestComposeUnmatchedTags(t *testing.T) {
	c := compose(t, testRoot(t), "empty")

	if len(c.Rules) != 0 {
		t.Errorf("rules = %v, want none", c.Rules)
	}
	want := []UnmatchedTag{{Profile: "empty", Tag: "missingtag"}}
	if !reflect.DeepEqual(c.UnmatchedTags, want) {
		t.Errorf("UnmatchedTags = %v, want %v", c.UnmatchedTags, want)
	}
}

// A typo must be attributed to the profile that contains it, not to the whole
// selection — otherwise a combined entry point gives you no file to go and fix.
func TestComposeAttributesUnmatchedTagsToTheirProfile(t *testing.T) {
	c := compose(t, testRoot(t), "python", "empty")

	want := []UnmatchedTag{{Profile: "empty", Tag: "missingtag"}}
	if !reflect.DeepEqual(c.UnmatchedTags, want) {
		t.Errorf("UnmatchedTags = %v, want %v", c.UnmatchedTags, want)
	}
}

// Several profiles compose by concatenating their tag selections in order. An
// item both select is loaded once, keeping the position of the earlier profile.
func TestComposeMultipleProfilesOrderAndDedup(t *testing.T) {
	root := testRoot(t)
	// "js" selects js and, second, general — which "python" already selected. The
	// shared general rules must not repeat, and must stay in the python block.
	write(t, root, "bootstrap/js.md",
		"---\ndescription: JS projects\nrules: [js, general]\nskills: [python]\n---\nJS preamble.")

	c := compose(t, root, "python", "js")

	want := []string{
		"rules/general/010-comm",  // general, from python
		"rules/python/020-shared", // general (tagged), from python
		"rules/python/010-errors", // python
		"rules/js/010-style",      // js, added by the second profile
	}
	if got := ids(c.Rules); !reflect.DeepEqual(got, want) {
		t.Errorf("rule order = %v, want %v", got, want)
	}

	// Both profiles select the "python" skill tag; it must appear exactly once.
	if got := ids(c.Skills); !reflect.DeepEqual(got, []string{"skills/python/release"}) {
		t.Errorf("skills = %v, want [skills/python/release]", got)
	}
}

// Reversing the order reverses the payload: the tag list drives the order, and
// naming a profile first puts its rules first.
func TestComposeMultipleProfilesRespectsGivenOrder(t *testing.T) {
	root := testRoot(t)
	write(t, root, "bootstrap/js.md",
		"---\ndescription: JS projects\nrules: [js]\n---\nJS preamble.")

	c := compose(t, root, "js", "python")

	want := []string{
		"rules/js/010-style",
		"rules/general/010-comm",
		"rules/python/020-shared",
		"rules/python/010-errors",
	}
	if got := ids(c.Rules); !reflect.DeepEqual(got, want) {
		t.Errorf("rule order = %v, want %v", got, want)
	}
}

// Every active profile's preamble reaches the agent; none is silently dropped.
func TestRenderEmitsEveryPreamble(t *testing.T) {
	root := testRoot(t)
	write(t, root, "bootstrap/js.md",
		"---\ndescription: JS projects\nrules: [js]\n---\nJS preamble.")

	out := compose(t, root, "python", "js").Render("get <id>")

	python := strings.Index(out, "Preamble text.")
	js := strings.Index(out, "JS preamble.")
	if python < 0 || js < 0 {
		t.Fatalf("a preamble is missing (python=%d js=%d):\n%s", python, js, out)
	}
	if python > js {
		t.Errorf("preambles are out of profile order:\n%s", out)
	}
}

func TestLoadNamedFailsOnAnyMissingProfile(t *testing.T) {
	root := testRoot(t)

	// The first name is valid, so a partial result would be easy to return by
	// accident — and would compose an entry point quietly missing a slice.
	if _, err := LoadNamed(root, []string{"python", "nope"}); err == nil {
		t.Error("LoadNamed returned nil error for a missing profile")
	}

	profiles, err := LoadNamed(root, []string{"python", "empty"})
	if err != nil {
		t.Fatalf("LoadNamed: %v", err)
	}
	if len(profiles) != 2 || profiles[0].Name != "python" || profiles[1].Name != "empty" {
		t.Errorf("LoadNamed did not preserve the given order: %v", profiles)
	}
}

func TestComposeIncludesLocalUnconditionally(t *testing.T) {
	root := testRoot(t)
	// A local overlay rule with a tag the "python" profile does NOT select, plus
	// a local skill. Both must appear regardless of tags.
	write(t, root, ".contour/rules/project-conventions.md", "---\ndescription: local rule\ntags: [unrelated]\n---\nlocal body")
	write(t, root, ".contour/skills/deploy/SKILL.md", "---\ndescription: local skill\n---\nsteps")

	p, err := LoadProfile(root, "python")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.LoadLayered(root, filepath.Join(root, ".contour"))
	if err != nil {
		t.Fatal(err)
	}
	c := Compose([]Profile{p}, st)

	if !hasID(c.Rules, "rules/project-conventions") {
		t.Errorf("local rule missing from eager rules: %v", ids(c.Rules))
	}
	if !hasID(c.Skills, "skills/deploy") {
		t.Errorf("local skill missing from menu: %v", ids(c.Skills))
	}

	out := c.Render("get <id>")
	if !strings.Contains(out, "# Project rules (local") {
		t.Errorf("Render lacks the local-rules section:\n%s", out)
	}
	if !strings.Contains(out, "take precedence") {
		t.Errorf("Render lacks the precedence note:\n%s", out)
	}
	if !strings.Contains(out, "(local)") {
		t.Errorf("Render lacks a (local) menu marker:\n%s", out)
	}
}

func ids(items []store.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func hasID(items []store.Item, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func TestRender(t *testing.T) {
	out := compose(t, testRoot(t), "python").Render("contour get <id>")

	for _, want := range []string{
		"Preamble text.",
		"# Rules",
		"## rules/general/010-comm",
		"Be concise.",
		"## rules/python/010-errors",
		"# Available skills",
		"- skills/python/release — release",
		"# Available knowledge",
		"- knowledge/general/stack — stack",
		"contour get <id>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render output missing %q\n---\n%s", want, out)
		}
	}

	if strings.Contains(out, "rules/js/010-style") {
		t.Error("Render included a rule the profile never selected")
	}
}

// bigRoot builds a store whose eager rules comfortably exceed any sane budget.
func bigRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "bootstrap/python.md",
		"---\ndescription: Python\nrules: [general, python]\nskills: [python]\n---\nPreamble.")
	for i := 1; i <= 6; i++ {
		body := strings.Repeat("Wrap every error with context so the trace survives. ", 12)
		write(t, root, fmt.Sprintf("rules/python/0%d0-r%d.md", i, i),
			fmt.Sprintf("---\ndescription: rule %d\n---\n%s", i, body))
	}
	write(t, root, "skills/python/release/SKILL.md", "---\ndescription: release\n---\nSteps")
	return root
}

// A payload that fits must be delivered byte-for-byte, with no notice warranted.
func TestRenderWithinPassesThroughWhenItFits(t *testing.T) {
	c := compose(t, testRoot(t), "python")
	full := c.Render("get <id>")

	ex := c.RenderWithin(len(full)+100, "get <id>")
	if !ex.Complete {
		t.Error("Complete = false for a payload that fits")
	}
	if ex.Body != full {
		t.Error("a fitting payload was altered")
	}
	if ex.ShownChars != ex.TotalChars || ex.ShownRules != ex.TotalRules {
		t.Errorf("counts disagree for a complete payload: %+v", ex)
	}
}

// The excerpt must actually respect the budget, and must report honest numbers.
func TestRenderWithinRespectsBudget(t *testing.T) {
	c := compose(t, bigRoot(t), "python")
	full := c.Render("get <id>")

	const budget = 900
	ex := c.RenderWithin(budget, "get <id>")

	if ex.Complete {
		t.Fatalf("Complete = true although the payload is %d chars", len(full))
	}
	if len(ex.Body) > budget {
		t.Errorf("body is %d chars, over the %d budget", len(ex.Body), budget)
	}
	if ex.ShownChars != len(ex.Body) {
		t.Errorf("ShownChars = %d, body is %d", ex.ShownChars, len(ex.Body))
	}
	if ex.TotalChars != len(full) {
		t.Errorf("TotalChars = %d, want %d (what `contour bootstrap` prints)", ex.TotalChars, len(full))
	}
	if ex.ShownRules >= ex.TotalRules || ex.ShownRules == 0 {
		t.Errorf("expected a partial rule set, got %d of %d", ex.ShownRules, ex.TotalRules)
	}
}

// Rules are included whole: a body cut mid-sentence can invert its own meaning,
// so every rule present must carry its complete text.
func TestRenderWithinKeepsRulesWhole(t *testing.T) {
	root := bigRoot(t)
	c := compose(t, root, "python")
	ex := c.RenderWithin(900, "get <id>")

	for _, it := range c.Rules {
		if !strings.Contains(ex.Body, "## "+it.ID) {
			continue // this rule was dropped, which is allowed
		}
		if !strings.Contains(ex.Body, it.Body) {
			t.Errorf("%s appears in the excerpt without its full body", it.ID)
		}
	}
}

// Local rules are authoritative on conflict. Dropping them in favour of the
// central rules they exist to override would make a session wrong, not merely
// short — so they get first claim on the budget.
func TestRenderWithinPrefersLocalRules(t *testing.T) {
	root := bigRoot(t)
	write(t, root, ".contour/rules/010-local.md",
		"---\ndescription: local\n---\nAlways use the project logger.")

	p, err := LoadProfile(root, "python")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.LoadLayered(root, filepath.Join(root, ".contour"))
	if err != nil {
		t.Fatal(err)
	}
	ex := Compose([]Profile{p}, st).RenderWithin(700, "get <id>")

	if ex.Complete {
		t.Fatal("expected an excerpt")
	}
	if !strings.Contains(ex.Body, "Always use the project logger.") {
		t.Errorf("the local rule was dropped in favour of central ones:\n%s", ex.Body)
	}
}

// Menus must never displace a rule. They are rendered only from what is left
// after the rules have taken what they need, because the list tool reproduces a
// menu on demand while a rule body has no other eager route to the agent.
func TestRenderWithinMenusNeverCostARule(t *testing.T) {
	const budget = 900

	lean := compose(t, bigRoot(t), "python")

	// The same store, but with a menu far too large for the leftover budget.
	root := bigRoot(t)
	for i := 1; i <= 12; i++ {
		write(t, root, fmt.Sprintf("skills/python/s%d/SKILL.md", i),
			fmt.Sprintf("---\ndescription: a reasonably wordy skill description number %d\n---\nSteps", i))
	}
	fat := compose(t, root, "python")

	a := lean.RenderWithin(budget, "get <id>")
	b := fat.RenderWithin(budget, "get <id>")

	if a.ShownRules != b.ShownRules {
		t.Errorf("menu size changed the rule count: %d with a small menu, %d with a large one",
			a.ShownRules, b.ShownRules)
	}
	if b.MenusIncluded {
		t.Error("a menu too large for the leftover budget was included")
	}
	if strings.Contains(b.Body, "# Available skills") {
		t.Error("MenusIncluded is false but a menu was rendered")
	}
	if len(b.Body) > budget {
		t.Errorf("body is %d chars, over the %d budget", len(b.Body), budget)
	}
}

// A budget too small even for one rule must still produce honest output rather
// than a half-rule or a panic.
func TestRenderWithinTinyBudget(t *testing.T) {
	c := compose(t, bigRoot(t), "python")
	ex := c.RenderWithin(10, "get <id>")

	if ex.Complete {
		t.Error("Complete = true for a 10-char budget")
	}
	if ex.ShownRules != 0 {
		t.Errorf("ShownRules = %d, want 0", ex.ShownRules)
	}
	if strings.Contains(ex.Body, "Wrap every error") {
		t.Error("a rule body leaked into an excerpt that cannot hold one")
	}
}

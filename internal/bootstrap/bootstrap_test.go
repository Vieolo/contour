package bootstrap

import (
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

func compose(t *testing.T, root, name string) Composed {
	t.Helper()
	p, err := LoadProfile(root, name)
	if err != nil {
		t.Fatalf("LoadProfile(%q): %v", name, err)
	}
	st, err := store.Load(root)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	return Compose(p, st)
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
	if want := []string{"missingtag"}; !reflect.DeepEqual(c.UnmatchedTags, want) {
		t.Errorf("UnmatchedTags = %v, want %v", c.UnmatchedTags, want)
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
	c := Compose(p, st)

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

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

// testRoot builds a store whose "go" profile exercises tag ordering and
// de-duplication: 20-shared lives under go/ but is also tagged general, so it
// matches both selected tags and must appear exactly once, in the general block.
func testRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write(t, root, "bootstrap/go.md",
		"---\ndescription: Go projects\nrules: [general, go]\nskills: [go]\nknowledge: [general]\n---\nPreamble text.")
	write(t, root, "bootstrap/empty.md",
		"---\ndescription: nothing matches\nrules: [missingtag]\n---\n")
	// Sorts after "go" by name, but before it by filename ('-' < '.').
	write(t, root, "bootstrap/go-frontend.md",
		"---\ndescription: Go plus frontend\nrules: [general, go]\n---\n")

	write(t, root, "rules/general/10-comm.md", "---\ndescription: comm\n---\nBe concise.")
	write(t, root, "rules/go/10-errors.md", "---\ndescription: errs\n---\nWrap errors.")
	write(t, root, "rules/go/20-shared.md", "---\ndescription: shared\ntags: [general]\n---\nShared rule.")
	write(t, root, "rules/js/10-style.md", "---\ndescription: style\n---\nUse prettier.")
	write(t, root, "skills/go/release/SKILL.md", "---\ndescription: release\n---\nSteps")
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

	p, err := LoadProfile(root, "go")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if p.Name != "go" {
		t.Errorf("Name = %q, want go", p.Name)
	}
	if p.Description != "Go projects" {
		t.Errorf("Description = %q", p.Description)
	}
	if p.Preamble != "Preamble text." {
		t.Errorf("Preamble = %q", p.Preamble)
	}
	if want := []string{"general", "go"}; !reflect.DeepEqual(p.RuleTags, want) {
		t.Errorf("RuleTags = %v, want %v", p.RuleTags, want)
	}
	if want := []string{"go"}; !reflect.DeepEqual(p.SkillTags, want) {
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
	if want := []string{"empty", "go", "go-frontend"}; !reflect.DeepEqual(names, want) {
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
	c := compose(t, testRoot(t), "go")

	var ids []string
	for _, it := range c.Rules {
		ids = append(ids, it.ID)
	}
	// general block first (10-comm, then 20-shared), then the go block adds
	// 10-errors; 20-shared is not repeated and js is never selected.
	want := []string{"rules/general/10-comm", "rules/go/20-shared", "rules/go/10-errors"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("rule order = %v, want %v", ids, want)
	}
}

func TestComposeLazyKinds(t *testing.T) {
	c := compose(t, testRoot(t), "go")

	if len(c.Skills) != 1 || c.Skills[0].ID != "skills/go/release" {
		t.Errorf("skills = %v, want [skills/go/release]", c.Skills)
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

func TestRender(t *testing.T) {
	out := compose(t, testRoot(t), "go").Render("contour get <id>")

	for _, want := range []string{
		"Preamble text.",
		"# Rules",
		"## rules/general/10-comm",
		"Be concise.",
		"## rules/go/10-errors",
		"# Available skills",
		"- skills/go/release — release",
		"# Available knowledge",
		"- knowledge/general/stack — stack",
		"contour get <id>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render output missing %q\n---\n%s", want, out)
		}
	}

	if strings.Contains(out, "rules/js/10-style") {
		t.Error("Render included a rule the profile never selected")
	}
}

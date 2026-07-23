package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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

func loadTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()

	write(t, root, "rules/general/010-comm.md", "---\ndescription: comm\n---\nBe concise.")
	write(t, root, "rules/go/010-errors.md", "---\ndescription: errs\ntags: [errors]\n---\nWrap errors.")
	write(t, root, "rules/go/web/020-http.md", "No frontmatter here.")
	write(t, root, "rules/go/notes.txt", "not markdown, ignored")
	write(t, root, "skills/go/release/SKILL.md", "---\ndescription: release\n---\n# Release\nsteps")
	write(t, root, "skills/go/release/helper.md", "asset inside a skill, not a separate item")
	write(t, root, "skills/deploy/SKILL.md", "# Deploy")
	write(t, root, "knowledge/general/stack.md", "---\ndescription: stack\n---\nGo + Postgres.")

	st, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return st
}

func TestLoadCounts(t *testing.T) {
	st := loadTestStore(t)
	if got := st.Count(KindRules); got != 3 {
		t.Errorf("rules = %d, want 3", got)
	}
	if got := st.Count(KindSkills); got != 2 {
		t.Errorf("skills = %d, want 2", got)
	}
	if got := st.Count(KindKnowledge); got != 1 {
		t.Errorf("knowledge = %d, want 1", got)
	}
}

func TestFrontmatterAndTags(t *testing.T) {
	st := loadTestStore(t)

	it, ok := st.Get("rules/go/010-errors")
	if !ok {
		t.Fatal("missing rules/go/010-errors")
	}
	if it.Description != "errs" {
		t.Errorf("description = %q, want errs", it.Description)
	}
	if want := []string{"go", "errors"}; !reflect.DeepEqual(it.Tags, want) {
		t.Errorf("tags = %v, want %v", it.Tags, want)
	}
	if it.Body != "Wrap errors." {
		t.Errorf("body = %q, want %q", it.Body, "Wrap errors.")
	}
	if it.Name != "010-errors" {
		t.Errorf("name = %q, want 010-errors", it.Name)
	}
}

func TestNoFrontmatter(t *testing.T) {
	st := loadTestStore(t)

	it, ok := st.Get("rules/go/web/020-http")
	if !ok {
		t.Fatal("missing rules/go/web/020-http")
	}
	if it.Description != "" {
		t.Errorf("description = %q, want empty", it.Description)
	}
	if want := []string{"go", "web"}; !reflect.DeepEqual(it.Tags, want) {
		t.Errorf("tags = %v, want %v", it.Tags, want)
	}
	if it.Body != "No frontmatter here." {
		t.Errorf("body = %q", it.Body)
	}
}

func TestSkills(t *testing.T) {
	st := loadTestStore(t)

	it, ok := st.Get("skills/go/release")
	if !ok {
		t.Fatal("missing skills/go/release")
	}
	if it.Kind != KindSkills {
		t.Errorf("kind = %v, want skills", it.Kind)
	}
	if it.Name != "release" {
		t.Errorf("name = %q, want release", it.Name)
	}
	if want := []string{"go"}; !reflect.DeepEqual(it.Tags, want) {
		t.Errorf("tags = %v, want %v", it.Tags, want)
	}
	if filepath.Base(it.Path) != SkillFile {
		t.Errorf("path = %q, want a %s", it.Path, SkillFile)
	}

	deploy, ok := st.Get("skills/deploy")
	if !ok {
		t.Fatal("missing skills/deploy")
	}
	if len(deploy.Tags) != 0 {
		t.Errorf("tags = %v, want none", deploy.Tags)
	}
}

func TestLoadOrdering(t *testing.T) {
	st := loadTestStore(t)

	var ids []string
	for _, it := range st.ByKind(KindRules) {
		ids = append(ids, it.ID)
	}
	want := []string{"rules/general/010-comm", "rules/go/010-errors", "rules/go/web/020-http"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("rule order = %v, want %v", ids, want)
	}
}

func TestGetMissing(t *testing.T) {
	st := loadTestStore(t)
	if _, ok := st.Get("rules/nope"); ok {
		t.Error("Get returned ok for a missing id")
	}
}

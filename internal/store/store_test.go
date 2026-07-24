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
	st, err := Load(testStoreRoot(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return st
}

func matchIDs(ms []Match) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Item.ID)
	}
	return out
}

func testStoreRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write(t, root, "rules/general/010-comm.md", "---\ndescription: comm\n---\nBe concise.")
	write(t, root, "rules/python/010-errors.md", "---\ndescription: errs\ntags: [errors]\n---\nWrap errors.")
	write(t, root, "rules/python/web/020-http.md", "No frontmatter here.")
	write(t, root, "rules/python/notes.txt", "not markdown, ignored")
	write(t, root, "skills/python/release/SKILL.md", "---\ndescription: release\n---\n# Release\nsteps")
	write(t, root, "skills/python/release/helper.md", "asset inside a skill, not a separate item")
	write(t, root, "skills/deploy/SKILL.md", "# Deploy")
	write(t, root, "knowledge/general/stack.md", "---\ndescription: stack\n---\nGo + Postgres.")

	return root
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

	it, ok := st.Get("rules/python/010-errors")
	if !ok {
		t.Fatal("missing rules/python/010-errors")
	}
	if it.Description != "errs" {
		t.Errorf("description = %q, want errs", it.Description)
	}
	if want := []string{"python", "errors"}; !reflect.DeepEqual(it.Tags, want) {
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

	it, ok := st.Get("rules/python/web/020-http")
	if !ok {
		t.Fatal("missing rules/python/web/020-http")
	}
	if it.Description != "" {
		t.Errorf("description = %q, want empty", it.Description)
	}
	if want := []string{"python", "web"}; !reflect.DeepEqual(it.Tags, want) {
		t.Errorf("tags = %v, want %v", it.Tags, want)
	}
	if it.Body != "No frontmatter here." {
		t.Errorf("body = %q", it.Body)
	}
}

func TestSkills(t *testing.T) {
	st := loadTestStore(t)

	it, ok := st.Get("skills/python/release")
	if !ok {
		t.Fatal("missing skills/python/release")
	}
	if it.Kind != KindSkills {
		t.Errorf("kind = %v, want skills", it.Kind)
	}
	if it.Name != "release" {
		t.Errorf("name = %q, want release", it.Name)
	}
	if want := []string{"python"}; !reflect.DeepEqual(it.Tags, want) {
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
	want := []string{"rules/general/010-comm", "rules/python/010-errors", "rules/python/web/020-http"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("rule order = %v, want %v", ids, want)
	}
}

func TestItemMatches(t *testing.T) {
	st := loadTestStore(t)

	it, ok := st.Get("rules/python/010-errors")
	if !ok {
		t.Fatal("missing rules/python/010-errors")
	}

	// Empty query matches everything; the rest hit the ID, description, tags
	// and body in turn, case-insensitively.
	for _, q := range []string{"", "ERRORS", "errs", "wrap errors", "python", "010-err"} {
		if !it.Matches(q) {
			t.Errorf("Matches(%q) = false, want true", q)
		}
	}
	if it.Matches("nonexistent-token") {
		t.Error(`Matches("nonexistent-token") = true, want false`)
	}
}

func TestStoreSearch(t *testing.T) {
	st := loadTestStore(t)

	hits := st.Search(KindRules, "concise")
	if len(hits) != 1 || hits[0].Item.ID != "rules/general/010-comm" {
		t.Errorf("Search(rules, concise) = %v, want [rules/general/010-comm]", matchIDs(hits))
	}
	if got := st.Search(KindSkills, "concise"); len(got) != 0 {
		t.Errorf("Search(skills, concise) = %v, want none", matchIDs(got))
	}
	if got := st.Search(KindRules, ""); len(got) != 3 {
		t.Errorf("Search(rules, empty) returned %d items, want 3", len(got))
	}
}

func TestSearchMatchDetail(t *testing.T) {
	st := loadTestStore(t)

	// A body match. "wrap" appears once in the errors rule body ("Wrap errors.")
	// and nowhere in its metadata.
	hits := st.Search(KindRules, "wrap")
	if len(hits) != 1 {
		t.Fatalf("Search(rules, wrap) = %d hits, want 1", len(hits))
	}
	m := hits[0]
	if m.Item.ID != "rules/python/010-errors" {
		t.Fatalf("id = %q", m.Item.ID)
	}
	if want := []string{"body"}; !reflect.DeepEqual(m.MatchedIn(), want) {
		t.Errorf("MatchedIn = %v, want %v", m.MatchedIn(), want)
	}
	if m.Occurrences != 1 {
		t.Errorf("Occurrences = %d, want 1", m.Occurrences)
	}
	if len(m.Lines) != 1 || m.Lines[0].Number != 1 {
		t.Errorf("Lines = %+v, want a single line numbered 1", m.Lines)
	}

	// A metadata-only match. "errs" is the description; it is not in the body.
	meta := st.Search(KindRules, "errs")
	if len(meta) != 1 {
		t.Fatalf("Search(rules, errs) = %d hits, want 1", len(meta))
	}
	if want := []string{"description"}; !reflect.DeepEqual(meta[0].MatchedIn(), want) {
		t.Errorf("MatchedIn = %v, want %v", meta[0].MatchedIn(), want)
	}
	if len(meta[0].Lines) != 0 {
		t.Errorf("a metadata match should carry no body lines, got %+v", meta[0].Lines)
	}
	if meta[0].Occurrences != 0 {
		t.Errorf("Occurrences = %d, want 0 for a metadata-only match", meta[0].Occurrences)
	}
}

func TestLoadForKind(t *testing.T) {
	root := testStoreRoot(t)

	_, kinds, err := LoadForKind(root, "")
	if err != nil {
		t.Fatalf("LoadForKind(all): %v", err)
	}
	if !reflect.DeepEqual(kinds, Kinds) {
		t.Errorf("kinds = %v, want %v", kinds, Kinds)
	}

	// Values arrive straight from a user, so trimming and case are handled.
	_, kinds, err = LoadForKind(root, "  Skills ")
	if err != nil {
		t.Fatalf("LoadForKind(skills): %v", err)
	}
	if want := []Kind{KindSkills}; !reflect.DeepEqual(kinds, want) {
		t.Errorf("kinds = %v, want %v", kinds, want)
	}

	if _, _, err := LoadForKind(root, "bogus"); err == nil {
		t.Error("LoadForKind(bogus) returned a nil error")
	}
}

func TestGetMissing(t *testing.T) {
	st := loadTestStore(t)
	if _, ok := st.Get("rules/nope"); ok {
		t.Error("Get returned ok for a missing id")
	}
}

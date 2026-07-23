package store

import "testing"

func TestSplitFrontmatter(t *testing.T) {
	fm, body, ok := SplitFrontmatter([]byte("---\ndescription: x\n---\nbody line"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if fm != "description: x" {
		t.Errorf("fm = %q", fm)
	}
	if body != "body line" {
		t.Errorf("body = %q", body)
	}
}

func TestSplitFrontmatterNone(t *testing.T) {
	_, body, ok := SplitFrontmatter([]byte("no frontmatter\nsecond line"))
	if ok {
		t.Error("ok = true, want false")
	}
	if body != "no frontmatter\nsecond line" {
		t.Errorf("body = %q", body)
	}
}

func TestSplitFrontmatterUnterminated(t *testing.T) {
	if _, _, ok := SplitFrontmatter([]byte("---\ndescription: x\nno close")); ok {
		t.Error("ok = true for unterminated frontmatter, want false")
	}
}

package scaffold

import (
	"os"
	"path/filepath"
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

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestMoveCarriesContentAndClearsSource(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "relocated")

	write(t, src, "rules/python/010-errors.md", "my own rule")
	write(t, src, "skills/python/release/SKILL.md", "my own skill")

	if err := Move(src, dst); err != nil {
		t.Fatalf("Move: %v", err)
	}

	if got := read(t, dst, "rules/python/010-errors.md"); got != "my own rule" {
		t.Errorf("moved rule = %q, want %q", got, "my own rule")
	}
	if got := read(t, dst, "skills/python/release/SKILL.md"); got != "my own skill" {
		t.Errorf("moved skill = %q", got)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source %s still exists after the move", src)
	}
}

func TestMoveCreatesMissingParents(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "a", "b", "store")

	write(t, src, "knowledge/general/stack.md", "content")

	if err := Move(src, dst); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got := read(t, dst, "knowledge/general/stack.md"); got != "content" {
		t.Errorf("content = %q, want %q", got, "content")
	}
}

func TestCopyTreePreservesContent(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "copy")
	write(t, src, "rules/general/010-comm.md", "be concise")

	if err := CopyTree(src, dst); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
	if got := read(t, dst, "rules/general/010-comm.md"); got != "be concise" {
		t.Errorf("copied content = %q", got)
	}
	// Unlike Move, the source survives.
	if got := read(t, src, "rules/general/010-comm.md"); got != "be concise" {
		t.Errorf("source content changed: %q", got)
	}
}

func TestCreateNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "my own words")

	created, err := Create(root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created {
		t.Error("Create reported creating a store over existing content")
	}
	if got := read(t, root, "README.md"); got != "my own words" {
		t.Errorf("README = %q, want the user's own content untouched", got)
	}
}

// A new store gets the full layout and every sample, whether the directory is
// missing outright or exists but is empty — `mkdir` first, `init` second is a
// perfectly reasonable way to set one up.
func TestCreateSeedsAMissingOrEmptyDirectory(t *testing.T) {
	for name, root := range map[string]string{
		"missing": filepath.Join(t.TempDir(), "store"),
		"empty":   t.TempDir(),
	} {
		t.Run(name, func(t *testing.T) {
			created, err := Create(root)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if !created {
				t.Fatal("Create reported no store created")
			}
			for _, f := range seedFiles() {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.rel))); err != nil {
					t.Errorf("sample %s missing: %v", f.rel, err)
				}
			}
		})
	}
}

// The complaint this guards against: re-running init on a store the user has
// already shaped must not put back what they removed. Samples are scaffolding for
// day one; once deleted, they stay deleted — and so does a kind directory the
// user has no use for.
func TestCreateLeavesAnExistingStoreAlone(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(root); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The user prunes the samples they don't want and drops a kind they don't use.
	deleted := "rules/python/010-errors.md"
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(deleted))); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "knowledge")); err != nil {
		t.Fatal(err)
	}

	created, err := Create(root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created {
		t.Error("Create reported creating a store where one already exists")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(deleted))); err == nil {
		t.Errorf("a deleted sample (%s) was put back", deleted)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge")); err == nil {
		t.Error("a deleted kind directory (knowledge/) was put back")
	}
}

// A file where the store should be can be neither a store nor a place to make
// one, so it must fail loudly rather than be silently treated as "has content".
func TestCreateRejectsAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store")
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(path); err == nil {
		t.Error("Create accepted a regular file as the store path")
	}
}

package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vieolo/contour/internal/store"
)

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func TestCreateLocalScaffoldsAgentsFolder(t *testing.T) {
	project := t.TempDir()

	overlay, created, err := CreateLocal(project)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !created {
		t.Fatal("created = false for a project with no local store")
	}
	if want := filepath.Join(project, store.DefaultOverlayDir); overlay != want {
		t.Errorf("overlay = %q, want %q", overlay, want)
	}
	for _, k := range store.Kinds {
		if !exists(t, filepath.Join(overlay, string(k), gitKeep)) {
			t.Errorf("%s/ is missing its %s, so git would not keep it", k, gitKeep)
		}
	}
	if !exists(t, filepath.Join(overlay, "README.md")) {
		t.Error("README.md missing")
	}
}

// The scaffold must add nothing to an agent's context. Local items are live the
// moment they exist — local rules load eagerly into every session — so if the
// README or a .gitkeep were ever read as an item, scaffolding would put
// placeholder text into real sessions.
func TestCreateLocalScaffoldContributesNoItems(t *testing.T) {
	project := t.TempDir()
	overlay, _, err := CreateLocal(project)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	st, err := store.LoadProject(t.TempDir(), []string{overlay}, nil)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if items := st.All(); len(items) != 0 {
		t.Errorf("the scaffold produced %d item(s), want none: %v", len(items), items)
	}
}

// A project that already has a local store, in any recognised overlay folder,
// is left alone — including one living in .claude/ rather than .agents/, so
// the project's local context is never split across two folders.
func TestCreateLocalLeavesAnExistingLocalStoreAlone(t *testing.T) {
	project := t.TempDir()
	write(t, project, ".claude/rules/010-mine.md", "my rule")

	overlay, created, err := CreateLocal(project)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if created {
		t.Error("created = true for a project that already has a local store")
	}
	if want := filepath.Join(project, ".claude"); overlay != want {
		t.Errorf("overlay = %q, want the existing %q", overlay, want)
	}
	if exists(t, filepath.Join(project, store.DefaultOverlayDir)) {
		t.Errorf("a second overlay (%s) was created beside the existing one", store.DefaultOverlayDir)
	}
}

// Re-running after pruning must not put back what the user removed — the same
// rule init follows for the central store.
func TestCreateLocalDoesNotRestoreADeletedKind(t *testing.T) {
	project := t.TempDir()
	overlay, _, err := CreateLocal(project)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(overlay, "knowledge")); err != nil {
		t.Fatal(err)
	}

	if _, created, err := CreateLocal(project); err != nil || created {
		t.Fatalf("re-run = (created %v, err %v), want an untouched no-op", created, err)
	}
	if exists(t, filepath.Join(overlay, "knowledge")) {
		t.Error("a deleted kind directory (knowledge/) was put back")
	}
}

// .agents/ is shared with other tools. A folder holding their files but no
// contour kinds is not a local store yet — scaffold into it, but never
// overwrite what is already there.
func TestCreateLocalSharesAnAgentsFolderWithOtherTools(t *testing.T) {
	project := t.TempDir()
	write(t, project, ".agents/README.md", "another tool's readme")
	write(t, project, ".agents/other-tool.json", "{}")

	overlay, created, err := CreateLocal(project)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !created {
		t.Fatal("created = false, but no contour kinds existed yet")
	}
	if got := read(t, project, ".agents/README.md"); got != "another tool's readme" {
		t.Errorf("README = %q, want the other tool's file left untouched", got)
	}
	if got := read(t, project, ".agents/other-tool.json"); got != "{}" {
		t.Errorf("other-tool.json = %q, want it untouched", got)
	}
	for _, k := range store.Kinds {
		if !exists(t, filepath.Join(overlay, string(k))) {
			t.Errorf("%s/ was not created", k)
		}
	}
}

// The scaffold includes a bootstrap/ folder for the project's own profiles, and
// an overlay holding only that already counts as a local store.
func TestCreateLocalScaffoldsProjectProfiles(t *testing.T) {
	project := t.TempDir()
	overlay, _, err := CreateLocal(project)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !exists(t, filepath.Join(overlay, store.BootstrapDir, gitKeep)) {
		t.Error("bootstrap/ was not scaffolded")
	}

	other := t.TempDir()
	write(t, other, ".contour/bootstrap/frontend.md", "---\nrules: [frontend]\n---\n")
	if got, created, err := CreateLocal(other); err != nil || created || got != filepath.Join(other, ".contour") {
		t.Errorf("CreateLocal = %q, created %v, %v; want the existing .contour left alone", got, created, err)
	}
}

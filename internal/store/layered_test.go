package store

import (
	"path/filepath"
	"testing"
)

func TestLoadLayeredMergesAndMarksSource(t *testing.T) {
	central := t.TempDir()
	overlay := filepath.Join(t.TempDir(), ".contour")

	write(t, central, "rules/python/010-errors.md", "---\ndescription: central errors\n---\ncentral body")
	write(t, central, "knowledge/general/stack.md", "central knowledge")
	write(t, overlay, "rules/project.md", "---\ndescription: project rule\n---\nlocal body")

	st, err := LoadLayered(central, overlay)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
	}

	central1, ok := st.Get("rules/python/010-errors")
	if !ok || central1.Source != OriginStore {
		t.Errorf("central item source = %v, want store", central1.Source)
	}
	local, ok := st.Get("rules/project")
	if !ok {
		t.Fatal("missing local rule rules/project")
	}
	if local.Source != OriginLocal {
		t.Errorf("local item source = %v, want local", local.Source)
	}
	if got := st.BySource(KindRules, OriginLocal); len(got) != 1 {
		t.Errorf("BySource(rules, local) = %d, want 1", len(got))
	}
}

func TestLoadLayeredLocalOverridesCentral(t *testing.T) {
	central := t.TempDir()
	overlay := filepath.Join(t.TempDir(), ".contour")

	// Identical path in both roots → same ID → local wins.
	write(t, central, "rules/errors.md", "---\ndescription: central\n---\ncentral body")
	write(t, overlay, "rules/errors.md", "---\ndescription: local\n---\nlocal body")

	st, err := LoadLayered(central, overlay)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
	}

	it, ok := st.Get("rules/errors")
	if !ok {
		t.Fatal("missing rules/errors")
	}
	if it.Source != OriginLocal || it.Body != "local body" {
		t.Errorf("override failed: source=%v body=%q, want local/%q", it.Source, it.Body, "local body")
	}
	// Overriding must not duplicate the item.
	if n := st.Count(KindRules); n != 1 {
		t.Errorf("rules count = %d after override, want 1", n)
	}
}

func TestLoadUnlayeredIsStoreSourced(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules/x.md", "body")

	st, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	it, _ := st.Get("rules/x")
	if it.Source != OriginStore {
		t.Errorf("source = %v, want store", it.Source)
	}
}

func TestDiscoverOverlays(t *testing.T) {
	project := t.TempDir()
	// Two of the three recognised folders present.
	write(t, project, ".contour/rules/a.md", "a")
	write(t, project, ".claude/rules/b.md", "b")

	got := DiscoverOverlays(project)
	if len(got) != 2 {
		t.Fatalf("found %v, want two overlays", got)
	}
	// Returned in OverlayDirNames order: .contour before .claude.
	if filepath.Base(got[0]) != ".contour" || filepath.Base(got[1]) != ".claude" {
		t.Errorf("order = %v, want [.contour .claude]", got)
	}
}

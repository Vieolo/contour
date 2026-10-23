package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// storeWithLocalRule builds a central store with one profile, "python", and a
// project overlay holding one local rule.
func storeWithLocalRule(t *testing.T) (root, overlay string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, root, "bootstrap/python.md", "---\nrules: [python]\n---\n")
	writeFile(t, root, "rules/python/010-errors.md", "Wrap errors.")
	overlay = filepath.Join(t.TempDir(), ".agents")
	writeFile(t, overlay, "rules/010-local.md", "Use the project logger.")
	return root, overlay
}

// A project names profiles from each developer's own store, so one developer
// missing one is ordinary. It must not take the server down — and with it the
// project's local rules, which have nothing to do with the missing profile.
func TestBuildInstructionsServesDespiteAMissingProfile(t *testing.T) {
	root, overlay := storeWithLocalRule(t)

	got, err := BuildInstructions(Options{
		Root:     root,
		Overlays: []string{overlay},
		Profiles: []string{"python", "go"},
	})
	if err != nil {
		t.Fatalf("BuildInstructions failed on a missing profile: %v", err)
	}
	for _, want := range []string{
		"Wrap errors.",            // the profile that exists still loads
		"Use the project logger.", // and so do the local rules
		"bootstrap profile go",    // and the agent is told what is missing
		"Mention this to the user",
	} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("instructions lack %q:\n%s", want, got.Text)
		}
	}
	// The notice goes above the rules, where a capping client cannot cut it.
	if strings.Index(got.Text, "bootstrap profile go") > strings.Index(got.Text, "Wrap errors.") {
		t.Error("the missing-profile notice comes after the rules")
	}
}

// With every profile missing, the session is served as if none were selected —
// local rules eager, the store on the menu — rather than empty.
func TestBuildInstructionsWithEveryProfileMissing(t *testing.T) {
	root, overlay := storeWithLocalRule(t)

	got, err := BuildInstructions(Options{
		Root:     root,
		Overlays: []string{overlay},
		Profiles: []string{"go"},
	})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	for _, want := range []string{
		"bootstrap profile go",
		"Use the project logger.", // local rules still eager
		"rules/python/010-errors", // central rules still reachable, on the menu
	} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("instructions lack %q:\n%s", want, got.Text)
		}
	}
}

func TestMissingProfilesNotice(t *testing.T) {
	if got := MissingProfilesNotice(nil); got != "" {
		t.Errorf("notice for nothing missing = %q, want empty", got)
	}
	if got := MissingProfilesNotice([]string{"go"}); !strings.Contains(got, "profile go, which is not") {
		t.Errorf("singular notice reads badly: %q", got)
	}
	if got := MissingProfilesNotice([]string{"go", "cli"}); !strings.Contains(got, "profiles go, cli, which are not") {
		t.Errorf("plural notice reads badly: %q", got)
	}
}

// The project's other presets are listed near the top, with their
// descriptions, so the agent knows what it can load and when. The session's own
// preset is not offered again.
func TestBuildInstructionsListsTheOtherPresets(t *testing.T) {
	root, overlay := storeWithLocalRule(t)

	got, err := BuildInstructions(Options{
		Root:     root,
		Overlays: []string{overlay},
		Profiles: []string{"python"},
		Preset:   "default",
		Presets: []PresetSummary{
			{Name: "default"},
			{Name: "frontend", Description: "The React app"},
			{Name: "backend"},
		},
	})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	for _, want := range []string{
		`started with the "default" preset`,
		"- frontend — The React app",
		"- backend\n",
		"`bootstrap` tool",
	} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("instructions lack %q:\n%s", want, got.Text)
		}
	}
	if strings.Contains(got.Text, "- default") {
		t.Error("the session's own preset is offered as something to load")
	}
	if strings.Index(got.Text, "# Presets") > strings.Index(got.Text, "Wrap errors.") {
		t.Error("the presets list comes after the rules, where a long payload would cut it")
	}
}

// With only one preset there is nothing to offer, and no section.
func TestBuildInstructionsOmitsPresetsWhenThereAreNoOthers(t *testing.T) {
	root, overlay := storeWithLocalRule(t)
	got, err := BuildInstructions(Options{
		Root: root, Overlays: []string{overlay}, Profiles: []string{"python"},
		Preset: "default", Presets: []PresetSummary{{Name: "default"}},
	})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	if strings.Contains(got.Text, "# Presets") {
		t.Errorf("a presets section with nothing in it:\n%s", got.Text)
	}
}

// RuleIDs names exactly the rules whose bodies the instructions carry — the ones
// a preset loaded later may leave out. An excerpt counts only what it shows.
func TestBuildInstructionsReportsTheRulesItDelivered(t *testing.T) {
	root, overlay := storeWithLocalRule(t)
	got, err := BuildInstructions(Options{Root: root, Overlays: []string{overlay}, Profiles: []string{"python"}})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	want := map[string]bool{"rules/python/010-errors": true, "rules/010-local": true}
	if len(got.RuleIDs) != len(want) {
		t.Fatalf("RuleIDs = %v, want %v", got.RuleIDs, want)
	}
	for _, id := range got.RuleIDs {
		if !want[id] {
			t.Errorf("RuleIDs has unexpected %q", id)
		}
	}

	// Push it over budget: only the rules actually shown may be claimed.
	for i := 0; i < 8; i++ {
		writeFile(t, root, "rules/python/02"+string(rune('0'+i))+"-long.md", strings.Repeat("A long rule body. ", 40))
	}
	ex, err := BuildInstructions(Options{Root: root, Overlays: []string{overlay}, Profiles: []string{"python"}})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	if !strings.Contains(ex.Text, "INCOMPLETE") {
		t.Fatal("expected an excerpt")
	}
	for _, id := range ex.RuleIDs {
		if !strings.Contains(ex.Text, "## "+id) {
			t.Errorf("RuleIDs claims %q, which the excerpt does not show", id)
		}
	}
}

package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vieolo/contour/internal/store"
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
	if got := MissingProfilesNotice(nil, true); got != "" {
		t.Errorf("notice for nothing missing = %q, want empty", got)
	}
	if got := MissingProfilesNotice([]string{"go"}, true); !strings.Contains(got, "profile go, which is in neither") {
		t.Errorf("singular notice reads badly: %q", got)
	}
	if got := MissingProfilesNotice([]string{"go", "cli"}, true); !strings.Contains(got, "profiles go, cli, which are in neither") {
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

// With local: selected and no profile, nothing of the project's is preloaded —
// not as a rule, not as a menu line — except the files the config names, which
// are always delivered. The rest stays reachable through list, search and get.
func TestBuildInstructionsLocalSelectedWithoutProfiles(t *testing.T) {
	root, overlay := storeWithLocalRule(t)
	notes := filepath.Join(t.TempDir(), "AGENTS.md")
	writeFile(t, filepath.Dir(notes), "AGENTS.md", "Repo notes.")

	got, err := BuildInstructions(Options{
		Root:          root,
		Overlays:      []string{overlay},
		EagerFiles:    []store.EagerFile{{ID: "AGENTS.md", Path: notes}},
		SelectedLocal: true,
	})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	if strings.Contains(got.Text, "Use the project logger.") {
		t.Errorf("an unselected project rule was delivered eagerly:\n%s", got.Text)
	}
	if strings.Contains(got.Text, "rules/010-local") {
		t.Errorf("an unselected project rule is on the startup menu:\n%s", got.Text)
	}
	if !strings.Contains(got.Text, "- rules/python/010-errors") {
		t.Errorf("the central rules left the menu too:\n%s", got.Text)
	}
	if !strings.Contains(got.Text, "Repo notes.") {
		t.Errorf("a named eager file was not delivered:\n%s", got.Text)
	}
}

// The eager rules and the menus that follow are separate sections; a heading
// must never run on from the last line of a rule.
func TestBuildInstructionsSeparatesRulesFromMenus(t *testing.T) {
	root, overlay := storeWithLocalRule(t)
	got, err := BuildInstructions(Options{Root: root, Overlays: []string{overlay}})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	if !strings.Contains(got.Text, "Use the project logger.\n\n# Available rules") {
		t.Errorf("the menu heading runs into the last rule:\n%s", got.Text)
	}
}

// With the central store left out (central: false), nothing of it reaches the
// session — no rule, no menu line — and a central profile the preset names is
// reported in words that point at the real fix: this preset cannot use it.
func TestBuildInstructionsWithoutTheCentralStore(t *testing.T) {
	_, overlay := storeWithLocalRule(t) // the central store exists, and is left out
	writeFile(t, overlay, "bootstrap/team.md", "---\nrules: [team]\n---\nTeam preamble.")
	writeFile(t, overlay, "rules/team/010-review.md", "Two approvals before merging.")

	got, err := BuildInstructions(Options{
		Root:     "", // central: false
		Overlays: []string{overlay},
		Profiles: []string{"team", "python"},
	})
	if err != nil {
		t.Fatalf("BuildInstructions: %v", err)
	}
	for _, want := range []string{"Team preamble.", "Two approvals before merging.", "Use the project logger.",
		"not among this project's own profiles", "access.central_store: false"} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("instructions lack %q:\n%s", want, got.Text)
		}
	}
	if strings.Contains(got.Text, "Wrap errors.") || strings.Contains(got.Text, "rules/python/010-errors") {
		t.Errorf("central content reached a session that leaves the central store out:\n%s", got.Text)
	}
}

func TestMissingProfilesNoticeWithoutTheCentralStore(t *testing.T) {
	got := MissingProfilesNotice([]string{"python"}, false)
	for _, want := range []string{"profile python, which is not among this project's own profiles", "add it to the project"} {
		if !strings.Contains(got, want) {
			t.Errorf("notice lacks %q: %q", want, got)
		}
	}
	if strings.Contains(got, "in neither") {
		t.Errorf("the local-only notice claims the profile does not exist: %q", got)
	}
}

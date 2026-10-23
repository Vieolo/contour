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
		if !strings.Contains(got, want) {
			t.Errorf("instructions lack %q:\n%s", want, got)
		}
	}
	// The notice goes above the rules, where a capping client cannot cut it.
	if strings.Index(got, "bootstrap profile go") > strings.Index(got, "Wrap errors.") {
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
		if !strings.Contains(got, want) {
			t.Errorf("instructions lack %q:\n%s", want, got)
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

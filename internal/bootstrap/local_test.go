package bootstrap

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vieolo/contour/internal/store"
)

// monorepo builds a central store and a project overlay laid out the way a
// monorepo keeps its context: repo-wide rules at the root of each kind, each
// part's under a folder named after it — which makes the folder a tag.
func monorepo(t *testing.T) (central, overlay string) {
	t.Helper()
	central = t.TempDir()
	write(t, central, "bootstrap/general.md", "---\ndescription: central general\nrules: [general]\n---\n")
	write(t, central, "bootstrap/frontend.md", "---\ndescription: CENTRAL frontend\nrules: [general]\n---\n")
	write(t, central, "rules/general/010-comm.md", "Be concise.")

	overlay = filepath.Join(t.TempDir(), ".agents")
	write(t, overlay, "bootstrap/frontend.md", "---\ndescription: project frontend\nrules: [frontend]\nskills: [frontend]\n---\nFrontend preamble.")
	write(t, overlay, "bootstrap/backend.md", "---\ndescription: project backend\nrules: [backend]\n---\n")
	write(t, overlay, "rules/010-repo-wide.md", "Run make before committing.")
	write(t, overlay, "rules/frontend/010-react.md", "Use function components.")
	write(t, overlay, "rules/backend/010-errors.md", "Wrap errors.")
	write(t, overlay, "skills/frontend/testing/SKILL.md", "---\ndescription: test a component\n---\nSteps")
	write(t, overlay, "skills/backend/migrate/SKILL.md", "---\ndescription: run a migration\n---\nSteps")
	return central, overlay
}

// A project profile takes the place of a central one with the same name — the
// project's own definition wins, as its items do — and says so.
func TestLoadProfileResolvesTheProjectFirst(t *testing.T) {
	central, overlay := monorepo(t)
	src := Sources{Central: central, Overlays: []string{overlay}}

	p, err := LoadProfile(src, "frontend")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if p.Description != "project frontend" || p.Source != store.OriginLocal || !p.Overrides {
		t.Errorf("frontend = %q from %s (overrides %v), want the project's, overriding the central one",
			p.Description, p.Source, p.Overrides)
	}

	p, err = LoadProfile(src, "backend")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if p.Source != store.OriginLocal || p.Overrides {
		t.Errorf("backend: source %s, overrides %v; want a project profile overriding nothing", p.Source, p.Overrides)
	}

	p, err = LoadProfile(src, "general")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if p.Source != store.OriginStore {
		t.Errorf("general came from %s, want the central store", p.Source)
	}
}

func TestLoadProfilesMergesBothSources(t *testing.T) {
	central, overlay := monorepo(t)
	profiles, err := LoadProfiles(Sources{Central: central, Overlays: []string{overlay}})
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}

	got := map[string]string{}
	var names []string
	for _, p := range profiles {
		got[p.Name] = p.Description
		names = append(names, p.Name)
	}
	if want := []string{"backend", "frontend", "general"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v (one each, by name)", names, want)
	}
	if got["frontend"] != "project frontend" {
		t.Errorf("frontend = %q, want the project's", got["frontend"])
	}
}

// Between two overlays, the first in precedence order wins.
func TestLoadProfilesFirstOverlayWins(t *testing.T) {
	first := filepath.Join(t.TempDir(), ".contour")
	second := filepath.Join(t.TempDir(), ".agents")
	write(t, first, "bootstrap/x.md", "---\ndescription: first\n---\n")
	write(t, second, "bootstrap/x.md", "---\ndescription: second\n---\n")
	src := Sources{Overlays: []string{first, second}}

	profiles, err := LoadProfiles(src)
	if err != nil || len(profiles) != 1 || profiles[0].Description != "first" {
		t.Errorf("LoadProfiles = %+v, %v; want the first overlay's", profiles, err)
	}
	if p, err := LoadProfile(src, "x"); err != nil || p.Description != "first" {
		t.Errorf("LoadProfile = %q, %v; want the first overlay's", p.Description, err)
	}
}

// Leaving the central store out leaves its profiles out — what a preset with
// central: false will ask for.
func TestSourcesWithoutTheCentralStore(t *testing.T) {
	_, overlay := monorepo(t)
	_, missing, err := LoadAvailable(Sources{Overlays: []string{overlay}}, []string{"frontend", "general"})
	if err != nil {
		t.Fatalf("LoadAvailable: %v", err)
	}
	if !reflect.DeepEqual(missing, []string{"general"}) {
		t.Errorf("missing = %v, want the central-only profile", missing)
	}
}

// With local: selected, project items are treated like central ones: in only
// if a profile's tags select them. The project's named files stay in.
func TestComposeSelectedTakesOnlyWhatTheProfilesSelect(t *testing.T) {
	central, overlay := monorepo(t)
	src := Sources{Central: central, Overlays: []string{overlay}}
	st, err := store.LoadProject(central, []string{overlay}, []store.EagerFile{
		{ID: "AGENTS.md", Path: writeAt(t, "AGENTS.md", "Repo notes.")},
	})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadNamed(src, []string{"general", "frontend"})
	if err != nil {
		t.Fatal(err)
	}

	c := Compose(profiles, st, false)

	if got, want := ids(c.Rules), []string{
		"rules/general/010-comm",   // central, by the general profile
		"rules/frontend/010-react", // project, by the frontend profile's tag
		"AGENTS.md",                // named in eager_files: in regardless
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %v, want %v", got, want)
	}
	if got := ids(c.Skills); !reflect.DeepEqual(got, []string{"skills/frontend/testing"}) {
		t.Errorf("skills = %v, want only the frontend one", got)
	}
	// The backend's items and the untagged repo-wide rule are left out — not
	// lost: list, search and get still reach them.
	for _, id := range []string{"rules/backend/010-errors", "rules/010-repo-wide", "skills/backend/migrate"} {
		if hasID(c.Rules, id) || hasID(c.Skills, id) {
			t.Errorf("%s was included though nothing selects it", id)
		}
	}
}

// local: all is unchanged: every project item, after the central ones the
// profiles select — whatever the profiles say about project tags.
func TestComposeAllLocalIncludesEveryProjectItem(t *testing.T) {
	central, overlay := monorepo(t)
	st, err := store.LoadProject(central, []string{overlay}, nil)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadNamed(Sources{Central: central, Overlays: []string{overlay}}, []string{"general"})
	if err != nil {
		t.Fatal(err)
	}

	c := Compose(profiles, st, true)
	for _, id := range []string{"rules/general/010-comm", "rules/010-repo-wide", "rules/frontend/010-react", "rules/backend/010-errors"} {
		if !hasID(c.Rules, id) {
			t.Errorf("%s missing under local: all", id)
		}
	}
}

// A tag only the project's items carry is not a typo — selecting the project's
// own folders is what a project profile is for.
func TestUnmatchedTagsCountProjectItems(t *testing.T) {
	central, overlay := monorepo(t)
	st, err := store.LoadProject(central, []string{overlay}, nil)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadNamed(Sources{Central: central, Overlays: []string{overlay}}, []string{"frontend"})
	if err != nil {
		t.Fatal(err)
	}
	if u := Compose(profiles, st, false).UnmatchedTags; len(u) != 0 {
		t.Errorf("UnmatchedTags = %v, want none: frontend matches project items", u)
	}
}

// The cross-reference reports profiles for project items too, and only calls
// them always active when some preset includes all project items.
func TestCrossReferenceProjectItems(t *testing.T) {
	central, overlay := monorepo(t)
	st, err := store.LoadProject(central, []string{overlay}, nil)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadProfiles(Sources{Central: central, Overlays: []string{overlay}})
	if err != nil {
		t.Fatal(err)
	}

	selected := crossRefOf(t, CrossReference(profiles, st, false))
	if got := selected["rules/frontend/010-react"].Profiles; !reflect.DeepEqual(got, []string{"frontend"}) {
		t.Errorf("react rule profiles = %v, want [frontend]", got)
	}
	if selected["rules/010-repo-wide"].Offered() {
		t.Error("an unselected project rule counts as offered though no preset includes all local items")
	}

	all := crossRefOf(t, CrossReference(profiles, st, true))
	if !all["rules/010-repo-wide"].Offered() {
		t.Error("a project rule is unoffered though a preset includes all local items")
	}
}

// writeAt writes a file in a fresh directory and returns its path.
func writeAt(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, name, content)
	return filepath.Join(dir, name)
}

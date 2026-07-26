package bootstrap

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/vieolo/contour/internal/store"
)

// crossRefOf indexes a cross-reference by item ID for direct assertions.
func crossRefOf(t *testing.T, cov CrossRef) map[string]ItemProfiles {
	t.Helper()
	byID := make(map[string]ItemProfiles, len(cov.Items))
	for _, ip := range cov.Items {
		byID[ip.Item.ID] = ip
	}
	return byID
}

func TestCrossReferenceNamesSelectingProfiles(t *testing.T) {
	root := testRoot(t)
	profiles, err := LoadProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	byID := crossRefOf(t, CrossReference(profiles, st))

	// LoadProfiles orders by name: empty, python, python-frontend. Both python
	// profiles select the "general" and "python" rule tags.
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"rules/general/010-comm", []string{"python", "python-frontend"}},
		{"rules/python/010-errors", []string{"python", "python-frontend"}},
		{"skills/python/release", []string{"python"}}, // only python selects skill tags
		{"knowledge/general/stack", []string{"python"}},
		{"rules/js/010-style", nil}, // no profile selects the js tag
	} {
		got := byID[tc.id]
		if !reflect.DeepEqual(got.Profiles, tc.want) {
			t.Errorf("%s: profiles = %v, want %v", tc.id, got.Profiles, tc.want)
		}
		if got.Offered() != (len(tc.want) > 0) {
			t.Errorf("%s: Offered() = %v", tc.id, got.Offered())
		}
	}
}

// The cross-reference is only worth reading if it says what a real session
// delivers. For every profile, the items it is credited with here must be
// exactly the central items Compose selects for it.
func TestCrossReferenceAgreesWithCompose(t *testing.T) {
	root := testRoot(t)
	write(t, root, "bootstrap/js.md",
		"---\ndescription: JS\nrules: [js]\nskills: [python]\nknowledge: [general]\n---\n")

	profiles, err := LoadProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cov := CrossReference(profiles, st)

	for _, p := range profiles {
		// What the cross-reference credits to this profile.
		var claimed []string
		for _, ip := range cov.Items {
			for _, name := range ip.Profiles {
				if name == p.Name {
					claimed = append(claimed, ip.Item.ID)
				}
			}
		}

		// What composing this profile alone actually delivers, central items only
		// (local items are delivered without being selected).
		c := Compose([]Profile{p}, st)
		var delivered []string
		for _, group := range [][]store.Item{c.Rules, c.Skills, c.Knowledge} {
			for _, it := range group {
				if it.Source == store.OriginStore {
					delivered = append(delivered, it.ID)
				}
			}
		}

		sort.Strings(claimed)
		sort.Strings(delivered)
		if !reflect.DeepEqual(claimed, delivered) {
			t.Errorf("profile %q: cross-reference claims %v, Compose delivers %v",
				p.Name, claimed, delivered)
		}
	}
}

// A local item is delivered without any profile selecting it, so it must never
// be reported as something no profile offers.
func TestCrossReferenceLocalItemsAreAlwaysActive(t *testing.T) {
	root := testRoot(t)
	write(t, root, ".contour/rules/project-conventions.md",
		"---\ndescription: local\ntags: [unrelated]\n---\nlocal body")

	profiles, err := LoadProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.LoadLayered(root, filepath.Join(root, ".contour"))
	if err != nil {
		t.Fatal(err)
	}

	local := crossRefOf(t, CrossReference(profiles, st))["rules/project-conventions"]
	if !local.AlwaysActive() {
		t.Error("a local item is not reported as always active")
	}
	if !local.Offered() {
		t.Error("a local item is not reported as offered")
	}
	if len(local.Profiles) != 0 {
		t.Errorf("a local item was credited to profiles %v", local.Profiles)
	}

	for _, ip := range CrossReference(profiles, st).Unoffered() {
		if ip.Item.ID == "rules/project-conventions" {
			t.Error("a local item was listed as offered by nothing")
		}
	}
}

func TestCoverageUnoffered(t *testing.T) {
	root := testRoot(t)
	profiles, err := LoadProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if got := ids2(CrossReference(profiles, st).Unoffered()); !reflect.DeepEqual(got, []string{"rules/js/010-style"}) {
		t.Errorf("Unoffered() = %v, want [rules/js/010-style]", got)
	}
}

// With no profiles at all, every central item is unoffered — the store is a
// library nothing has an entry point into.
func TestCrossReferenceNoProfiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules/general/010-comm.md", "---\ndescription: comm\n---\nBe concise.")

	st, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cov := CrossReference(nil, st)

	if got := ids2(cov.Unoffered()); !reflect.DeepEqual(got, []string{"rules/general/010-comm"}) {
		t.Errorf("Unoffered() = %v, want the one item", got)
	}
}

func TestCoverageByKind(t *testing.T) {
	root := testRoot(t)
	profiles, err := LoadProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cov := CrossReference(profiles, st)

	for _, k := range store.Kinds {
		for _, ip := range cov.ByKind(k) {
			if ip.Item.Kind != k {
				t.Errorf("ByKind(%s) returned a %s item", k, ip.Item.Kind)
			}
		}
	}
	if n := len(cov.ByKind(store.KindSkills)); n != 1 {
		t.Errorf("ByKind(skills) = %d items, want 1", n)
	}
}

func ids2(items []ItemProfiles) []string {
	var out []string
	for _, ip := range items {
		out = append(out, ip.Item.ID)
	}
	return out
}

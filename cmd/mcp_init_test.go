package cmd

import (
	"reflect"
	"testing"

	"github.com/vieolo/contour/internal/bootstrap"
	"github.com/vieolo/termange/tui"
)

// The picker shows each profile with its description — that is what someone who
// has forgotten the names will recognise — but must hand back bare names, since
// those are what the project config records.
func TestProfileChoicesLabelWithDescriptionAndMapBackToNames(t *testing.T) {
	profiles := []bootstrap.Profile{
		{Name: "cli", Description: "Projects that ship a CLI"},
		{Name: "python", Description: "Python backend projects"},
		{Name: "bare"}, // no description: the name alone
	}

	items, nameOf := profileChoices(profiles)

	wantItems := []tui.SelectItem{
		"cli — Projects that ship a CLI",
		"python — Python backend projects",
		"bare",
	}
	if !reflect.DeepEqual(items, wantItems) {
		t.Errorf("items = %q, want %q", items, wantItems)
	}
	// The picker returns its selection in item order, so item order is the
	// order the chosen profiles compose in; it must follow the input.
	for i, it := range items {
		if got := nameOf[string(it)]; got != profiles[i].Name {
			t.Errorf("label %q maps to %q, want %q", it, got, profiles[i].Name)
		}
	}
}

//go:build dev

package config

import "testing"

func TestActiveProfileIsDevelopment(t *testing.T) {
	if !Dev {
		t.Error("Dev = false, want true in a development build")
	}
	if EnvVar != "CONTOUR_HOME_DEV" {
		t.Errorf("EnvVar = %q, want CONTOUR_HOME_DEV", EnvVar)
	}
	if Label != "development" {
		t.Errorf("Label = %q, want development", Label)
	}
	if Program != "contour-dev" {
		t.Errorf("Program = %q, want contour-dev", Program)
	}
	if active.defaultDirName != ".contour-dev" {
		t.Errorf("defaultDirName = %q, want .contour-dev", active.defaultDirName)
	}
}

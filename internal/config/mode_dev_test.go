//go:build dev

package config

import "testing"

func TestActiveProfileIsDevelopment(t *testing.T) {
	if !Dev {
		t.Error("Dev = false, want true in a development build")
	}
	if Label != "development" {
		t.Errorf("Label = %q, want development", Label)
	}
	if Program != "contour-dev" {
		t.Errorf("Program = %q, want contour-dev", Program)
	}
	// The build tag alone provides dev/production isolation: a different default
	// store and a different config file, with no environment variable involved.
	if active.defaultDirName != "contour-dev" {
		t.Errorf("defaultDirName = %q, want contour-dev", active.defaultDirName)
	}
	if active.configFileName != "config-dev.yaml" {
		t.Errorf("configFileName = %q, want config-dev.yaml", active.configFileName)
	}
}

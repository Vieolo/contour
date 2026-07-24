//go:build !dev

package config

import "testing"

func TestActiveProfileIsProduction(t *testing.T) {
	if Dev {
		t.Error("Dev = true, want false in a production build")
	}
	if Label != "production" {
		t.Errorf("Label = %q, want production", Label)
	}
	if Program != "contour" {
		t.Errorf("Program = %q, want contour", Program)
	}
	if active.defaultDirName != "contour" {
		t.Errorf("defaultDirName = %q, want contour", active.defaultDirName)
	}
	if active.configFileName != "config.yaml" {
		t.Errorf("configFileName = %q, want config.yaml", active.configFileName)
	}
}

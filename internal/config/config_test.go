package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExplicitExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvVar, dir)

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !h.Explicit {
		t.Error("Explicit = false, want true")
	}
	if !h.Exists {
		t.Error("Exists = false, want true")
	}
	if h.Path != dir {
		t.Errorf("Path = %q, want %q", h.Path, dir)
	}
}

func TestResolveExplicitMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nope")
	t.Setenv(EnvVar, dir)

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !h.Explicit {
		t.Error("Explicit = false, want true")
	}
	if h.Exists {
		t.Error("Exists = true, want false")
	}
}

func TestResolveDefaultFromHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvVar, "") // ensure the override is unset

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if h.Explicit {
		t.Error("Explicit = true, want false")
	}
	want := filepath.Join(home, active.defaultDirName)
	if h.Path != want {
		t.Errorf("Path = %q, want %q", h.Path, want)
	}
	if h.Exists {
		t.Error("Exists = true, want false (dir not created)")
	}

	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	h2, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve after mkdir: %v", err)
	}
	if !h2.Exists {
		t.Error("Exists = false after mkdir, want true")
	}
}

func TestResolveTildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvVar, "~/mystore")

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(home, "mystore")
	if h.Path != want {
		t.Errorf("Path = %q, want %q", h.Path, want)
	}
}

func TestResolveRelativeBecomesAbsolute(t *testing.T) {
	t.Setenv(EnvVar, "relative/store")

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !filepath.IsAbs(h.Path) {
		t.Errorf("Path = %q, want absolute", h.Path)
	}
}

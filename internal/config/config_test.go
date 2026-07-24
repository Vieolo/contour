package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// isolate points HOME at a fresh temp directory so a test never reads or writes
// the developer's real store or config file. Both the default store (~/contour)
// and the config directory (~/.contour) hang off HOME, so one lever isolates
// everything — contour itself reads no environment variables.
func isolate(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestResolveDefault(t *testing.T) {
	home := isolate(t)

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(home, active.defaultDirName); h.Path != want {
		t.Errorf("Path = %q, want %q", h.Path, want)
	}
	if h.Source != SourceDefault {
		t.Errorf("Source = %q, want %q", h.Source, SourceDefault)
	}
	if h.Explicit {
		t.Error("Explicit = true, want false for the default location")
	}
	if h.Exists {
		t.Error("Exists = true, want false (resolving must not create anything)")
	}
}

func TestResolveFromConfigFile(t *testing.T) {
	isolate(t)
	store := t.TempDir()

	gotPath, configFile, err := SetStorePath(store)
	if err != nil {
		t.Fatalf("SetStorePath: %v", err)
	}
	if gotPath != store {
		t.Errorf("stored path = %q, want %q", gotPath, store)
	}

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if h.Path != store {
		t.Errorf("Path = %q, want %q", h.Path, store)
	}
	if h.Source != SourceConfig {
		t.Errorf("Source = %q, want %q", h.Source, SourceConfig)
	}
	if !h.Explicit {
		t.Error("Explicit = false, want true for a configured location")
	}
	if !h.Exists {
		t.Error("Exists = false, want true")
	}

	// The whole point of the config file: moving the store must not carry the
	// pointer away with it.
	if strings.HasPrefix(configFile, store+string(filepath.Separator)) {
		t.Errorf("config file %q lives inside the store %q", configFile, store)
	}
}

func TestSetStorePathReplacesPrevious(t *testing.T) {
	isolate(t)
	first, second := t.TempDir(), t.TempDir()

	if _, _, err := SetStorePath(first); err != nil {
		t.Fatalf("SetStorePath(first): %v", err)
	}
	if _, _, err := SetStorePath(second); err != nil {
		t.Fatalf("SetStorePath(second): %v", err)
	}

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if h.Path != second {
		t.Errorf("Path = %q, want %q", h.Path, second)
	}
}

func TestResolveConfiguredMissing(t *testing.T) {
	isolate(t)
	missing := filepath.Join(t.TempDir(), "nope")

	if _, _, err := SetStorePath(missing); err != nil {
		t.Fatalf("SetStorePath: %v", err)
	}

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

func TestSetStorePathExpandsTilde(t *testing.T) {
	home := isolate(t)

	if _, _, err := SetStorePath("~/mystore"); err != nil {
		t.Fatalf("SetStorePath: %v", err)
	}

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(home, "mystore"); h.Path != want {
		t.Errorf("Path = %q, want %q", h.Path, want)
	}
}

func TestSetStorePathMakesRelativeAbsolute(t *testing.T) {
	isolate(t)

	if _, _, err := SetStorePath("relative/store"); err != nil {
		t.Fatalf("SetStorePath: %v", err)
	}

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !filepath.IsAbs(h.Path) {
		t.Errorf("Path = %q, want absolute", h.Path)
	}
}

func TestConfigPathIsFixedAndOutsideTheStore(t *testing.T) {
	home := isolate(t)

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	if want := filepath.Join(home, ".contour", active.configFileName); path != want {
		t.Errorf("ConfigPath = %q, want %q", path, want)
	}

	// The config must not sit inside the default store, or relocating the store
	// by moving the directory would take the pointer with it.
	store, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if strings.HasPrefix(path, store.Path+string(filepath.Separator)) {
		t.Errorf("config %q lives inside the store %q", path, store.Path)
	}
}

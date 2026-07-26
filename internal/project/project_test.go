package project

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReadsBootstrapAndEagerFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, FileName),
		"bootstrap: python\neager_files:\n  - AGENTS.md\n  - docs/arch.md\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bootstrap != "python" {
		t.Errorf("Bootstrap = %q, want python", cfg.Bootstrap)
	}
	if len(cfg.EagerFiles) != 2 {
		t.Fatalf("EagerFiles = %v, want 2", cfg.EagerFiles)
	}
	// ID keeps the listed path; Path is resolved to an absolute location.
	if cfg.EagerFiles[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles[0].ID = %q, want AGENTS.md", cfg.EagerFiles[0].ID)
	}
	if want := filepath.Join(dir, "AGENTS.md"); cfg.EagerFiles[0].Path != want {
		t.Errorf("EagerFiles[0].Path = %q, want %q", cfg.EagerFiles[0].Path, want)
	}
	if want := filepath.Join(dir, "docs", "arch.md"); cfg.EagerFiles[1].Path != want {
		t.Errorf("EagerFiles[1].Path = %q, want %q", cfg.EagerFiles[1].Path, want)
	}
}

func TestLoadMissingConfigIsZero(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bootstrap != "" || len(cfg.EagerFiles) != 0 || cfg.Path != "" {
		t.Errorf("expected a zero Config, got %+v", cfg)
	}
}

func TestWriteRoundtripsAtProjectRoot(t *testing.T) {
	dir := t.TempDir()

	path, err := Write(dir, "python", []string{"AGENTS.md"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	// The config must sit at the project root, not inside any overlay folder, so
	// moving local rules between overlay folders never loses it.
	if want := filepath.Join(dir, FileName); path != want {
		t.Errorf("Write path = %q, want %q (project root)", path, want)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bootstrap != "python" {
		t.Errorf("Bootstrap = %q, want python", cfg.Bootstrap)
	}
	if len(cfg.EagerFiles) != 1 || cfg.EagerFiles[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles = %v, want [AGENTS.md]", cfg.EagerFiles)
	}
}

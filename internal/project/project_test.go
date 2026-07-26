package project

import (
	"os"
	"path/filepath"
	"reflect"
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
	// A bare name is the pre-existing shape and must keep working unchanged.
	if want := []string{"python"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("Bootstrap = %v, want %v", cfg.Bootstrap, want)
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

// Several profiles compose into one entry point, so the key takes a list too.
func TestLoadReadsBootstrapList(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want []string
	}{
		{"flow", "bootstrap: [python, cli]\n", []string{"python", "cli"}},
		{"block", "bootstrap:\n  - python\n  - cli\n", []string{"python", "cli"}},
		// Order is meaningful, so de-duplication keeps the first occurrence.
		{"dedup", "bootstrap: [python, cli, python]\n", []string{"python", "cli"}},
		{"blanks", "bootstrap: [python, \"\", \"  cli \"]\n", []string{"python", "cli"}},
		{"empty", "bootstrap: []\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, FileName), tc.yaml)

			cfg, err := Load(dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !reflect.DeepEqual(cfg.Bootstrap, tc.want) {
				t.Errorf("Bootstrap = %v, want %v", cfg.Bootstrap, tc.want)
			}
		})
	}
}

func TestLoadRejectsNonNameBootstrap(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, FileName), "bootstrap:\n  name: python\n")

	if _, err := Load(dir); err == nil {
		t.Error("Load accepted a mapping for bootstrap")
	}
}

func TestLoadMissingConfigIsZero(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Bootstrap) != 0 || len(cfg.EagerFiles) != 0 || cfg.Path != "" {
		t.Errorf("expected a zero Config, got %+v", cfg)
	}
}

func TestWriteRoundtripsAtProjectRoot(t *testing.T) {
	dir := t.TempDir()

	path, err := Write(dir, []string{"python"}, []string{"AGENTS.md"})
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
	if want := []string{"python"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("Bootstrap = %v, want %v", cfg.Bootstrap, want)
	}
	if len(cfg.EagerFiles) != 1 || cfg.EagerFiles[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles = %v, want [AGENTS.md]", cfg.EagerFiles)
	}
}

// What Write emits must be what Load accepts, for every profile count — the
// written file is the one a user then edits by hand.
func TestWriteRoundtripsProfileCounts(t *testing.T) {
	for _, want := range [][]string{nil, {"python"}, {"python", "cli"}} {
		dir := t.TempDir()
		if _, err := Write(dir, want, nil); err != nil {
			t.Fatalf("Write(%v): %v", want, err)
		}
		cfg, err := Load(dir)
		if err != nil {
			t.Fatalf("Load after Write(%v): %v", want, err)
		}
		if !reflect.DeepEqual(cfg.Bootstrap, want) {
			t.Errorf("round trip of %v gave %v", want, cfg.Bootstrap)
		}
	}
}

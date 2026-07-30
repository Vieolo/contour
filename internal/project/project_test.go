package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

const goYAML = "name: myproject\ndescription: a thing\nversion: 1.2.3\n\n" +
	"external:\n  gomore:\n    commands:\n      build: \"go build ./...\"\n"

// A project that keeps a go.yaml can hold contour's settings in it.
func TestLoadReadsGoYAMLExternalSection(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML+
		"  contour:\n    bootstrap: [python, cli]\n    eager_files:\n      - AGENTS.md\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"python", "cli"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("Bootstrap = %v, want %v", cfg.Bootstrap, want)
	}
	if len(cfg.EagerFiles) != 1 || cfg.EagerFiles[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles = %v, want [AGENTS.md]", cfg.EagerFiles)
	}
	if cfg.Path != filepath.Join(dir, GoYAMLFile) {
		t.Errorf("Path = %q, want the go.yaml", cfg.Path)
	}
	// The scalar form must work here too, not just in .contour.yaml.
	write(t, filepath.Join(dir, GoYAMLFile), goYAML+"  contour:\n    bootstrap: python\n")
	cfg, err = Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"python"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("scalar bootstrap = %v, want %v", cfg.Bootstrap, want)
	}
}

// A project keeps its go.yaml for reasons of its own. One without a contour
// section must not be mistaken for a contour config, nor disturb the
// .contour.yaml beside it.
func TestLoadIgnoresGoYAMLWithoutContourSection(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML)
	write(t, filepath.Join(dir, FileName), "bootstrap: [python]\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"python"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("Bootstrap = %v, want %v from .contour.yaml", cfg.Bootstrap, want)
	}
	if cfg.Path != filepath.Join(dir, FileName) {
		t.Errorf("Path = %q, want the .contour.yaml", cfg.Path)
	}
	if len(cfg.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", cfg.Warnings)
	}
}

// With settings in both, contour's own file wins — it is the unambiguous
// statement of intent, and the rule holds however many host manifests exist.
// The shadowed host section is named rather than silently ignored.
func TestLoadPrefersOwnFileOverHostAndWarns(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML+"  contour:\n    bootstrap: [fromgo]\n")
	write(t, filepath.Join(dir, FileName), "bootstrap: [fromown]\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"fromown"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("Bootstrap = %v, want %v (.contour.yaml wins)", cfg.Bootstrap, want)
	}
	if cfg.Path != filepath.Join(dir, FileName) {
		t.Errorf("Path = %q, want the .contour.yaml", cfg.Path)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], GoYAMLFile) {
		t.Errorf("expected a warning naming %s, got %v", GoYAMLFile, cfg.Warnings)
	}
}

// A go.yaml contour cannot parse belongs to the project, not to contour.
// It must degrade to a warning and the fallback, not break every command.
func TestLoadToleratesUnreadableGoYAML(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), "name: [unclosed\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an error for a malformed go.yaml: %v", err)
	}
	if len(cfg.Bootstrap) != 0 {
		t.Errorf("Bootstrap = %v, want none", cfg.Bootstrap)
	}
	if len(cfg.Warnings) != 1 {
		t.Errorf("expected one warning, got %v", cfg.Warnings)
	}
}

// Write targets go.yaml when the project has one and no .contour.yaml, sparing
// it a second config file. The result round-trips.
func TestWriteTargetsGoYAMLWhenPresent(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML)

	path, err := Write(dir, []string{"python", "cli"}, []string{"AGENTS.md"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if path != filepath.Join(dir, GoYAMLFile) {
		t.Errorf("Write path = %q, want the go.yaml", path)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
		t.Error("Write created a .contour.yaml even though go.yaml exists")
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"python", "cli"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("round trip gave %v, want %v", cfg.Bootstrap, want)
	}
	if len(cfg.EagerFiles) != 1 || cfg.EagerFiles[0].ID != "AGENTS.md" {
		t.Errorf("round trip gave EagerFiles %v", cfg.EagerFiles)
	}
}

// Writing contour's section must leave the rest of a project's go.yaml alone —
// its metadata, another tool's config, and the ordering of both.
func TestWritePreservesTheRestOfGoYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, GoYAMLFile)
	write(t, path, goYAML)

	if _, err := Write(dir, []string{"python"}, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"name: myproject", "description: a thing", "version: 1.2.3",
		"gomore:", "build: \"go build ./...\"", "contour:",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("go.yaml lost %q after writing contour's section:\n%s", want, got)
		}
	}
	// contour's section must sit under external, beside the other tool's.
	if strings.Index(string(got), "external:") > strings.Index(string(got), "contour:") {
		t.Errorf("contour's section is not under external:\n%s", got)
	}
}

// Overwriting an existing contour section must replace it, not append a second.
func TestWriteReplacesAnExistingContourSection(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML+"  contour:\n    bootstrap: [old]\n")

	if _, err := Write(dir, []string{"new"}, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, GoYAMLFile))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(got), "contour:"); n != 1 {
		t.Errorf("go.yaml has %d contour sections, want 1:\n%s", n, got)
	}
	if strings.Contains(string(got), "old") {
		t.Errorf("the previous bootstrap survived:\n%s", got)
	}
}

// An existing .contour.yaml is what Load reads, so it is what Write must
// update. Writing into the host manifest instead would file the settings
// somewhere permanently shadowed.
func TestWritePrefersAnExistingOwnFileOverAHost(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML)
	write(t, filepath.Join(dir, FileName), "bootstrap: [old]\n")

	path, err := Write(dir, []string{"new"}, nil)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if path != filepath.Join(dir, FileName) {
		t.Errorf("Write path = %q, want the existing .contour.yaml", path)
	}

	got, err := os.ReadFile(filepath.Join(dir, GoYAMLFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "contour:") {
		t.Errorf("Write added a shadowed section to go.yaml:\n%s", got)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"new"}; !reflect.DeepEqual(cfg.Bootstrap, want) {
		t.Errorf("round trip gave %v, want %v", cfg.Bootstrap, want)
	}
}

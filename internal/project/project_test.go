package project

import (
	"errors"
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// unchanged fails the test if the file at path no longer holds want, byte for
// byte. An existing config must not merely parse back to the same values — it
// must not be rewritten at all, or a teammate's comments and formatting go.
func unchanged(t *testing.T, path, want string) {
	t.Helper()
	if got := readFile(t, path); got != want {
		t.Errorf("%s was modified:\n--- want\n%s\n--- got\n%s", filepath.Base(path), want, got)
	}
}

// preset fetches a preset or fails the test.
func preset(t *testing.T, cfg Config, name string) Preset {
	t.Helper()
	p, ok := cfg.Preset(name)
	if !ok {
		t.Fatalf("no preset %q in %v", name, cfg.Names())
	}
	return p
}

const goYAML = "name: myproject\ndescription: a thing\nversion: 1.2.3\n\n" +
	"external:\n  gomore:\n    commands:\n      build: \"go build ./...\"\n"

// --- reading presets ---------------------------------------------------------

func TestLoadReadsPresetsInFileOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, FileName), `presets:
  default:
    startup:
      profiles: [general]
      files:
        - AGENTS.md
  frontend:
    description: The React app
    startup:
      profiles: [general, react]
      files: [frontend/AGENTS.md]
      local_store: selected
    access:
      central_store: false
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"default", "frontend"}; !reflect.DeepEqual(cfg.Names(), want) {
		t.Errorf("Names = %v, want %v (file order)", cfg.Names(), want)
	}

	def := preset(t, cfg, "default")
	if want := []string{"general"}; !reflect.DeepEqual(def.Profiles, want) {
		t.Errorf("default.Profiles = %v, want %v", def.Profiles, want)
	}
	if len(def.Files) != 1 || def.Files[0].ID != "AGENTS.md" ||
		def.Files[0].Path != filepath.Join(dir, "AGENTS.md") {
		t.Errorf("default.Files = %v, want AGENTS.md resolved against the project", def.Files)
	}
	// Unset settings take their defaults.
	if def.LocalStore != LocalAll || !def.CentralStore {
		t.Errorf("default settings = local %q central %v, want all/true", def.LocalStore, def.CentralStore)
	}

	fe := preset(t, cfg, "frontend")
	if fe.Description != "The React app" {
		t.Errorf("frontend.Description = %q", fe.Description)
	}
	if fe.LocalStore != LocalSelected || fe.CentralStore {
		t.Errorf("frontend settings = local %q central %v, want selected/false", fe.LocalStore, fe.CentralStore)
	}
	if want := filepath.Join(dir, "frontend", "AGENTS.md"); len(fe.Files) != 1 || fe.Files[0].Path != want {
		t.Errorf("frontend.Files = %v, want %s", fe.Files, want)
	}
}

// startup.profiles takes one name or a list; order counts and repeats drop.
func TestLoadReadsBootstrapShapes(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        []string
	}{
		{"scalar", "python", []string{"python"}},
		{"flow", "[python, cli]", []string{"python", "cli"}},
		{"block", "\n        - python\n        - cli", []string{"python", "cli"}},
		{"dedup", "[python, cli, python]", []string{"python", "cli"}},
		{"blanks", "[python, \"\", \"  cli \"]", []string{"python", "cli"}},
		{"empty", "[]", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, FileName), "presets:\n  default:\n    startup:\n      profiles: "+tc.value+"\n")
			cfg, err := Load(dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := preset(t, cfg, DefaultPreset).Profiles; !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Bootstrap = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadRejectsInvalidPresets(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"mapping as profiles", "presets:\n  default:\n    startup:\n      profiles:\n        name: python\n"},
		{"unknown local_store", "presets:\n  default:\n    startup:\n      local_store: some\n"},
		{"central_store not a boolean", "presets:\n  default:\n    access:\n      central_store: nope\n"},
		{"startup not a mapping", "presets:\n  default:\n    startup: [python]\n"},
		{"access not a mapping", "presets:\n  default:\n    access: false\n"},
		{"presets not a mapping", "presets: [default]\n"},
		{"preset not a mapping", "presets:\n  default: python\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, FileName), tc.body)
			if _, err := Load(dir); err == nil {
				t.Error("Load accepted an invalid config")
			}
		})
	}
}

// A typo in a key would otherwise leave a preset silently loading nothing — at
// any level: the root, a preset, or inside its startup and access groups.
func TestLoadWarnsAboutUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, FileName), `presets:
  default:
    eager_files: [AGENTS.md]
    startup:
      file: [AGENTS.md]
    access:
      central: false
profile: x
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	joined := strings.Join(cfg.Warnings, "\n")
	for _, want := range []string{
		`preset default: unknown key "eager_files"`,
		`preset default: startup: unknown key "file"`,
		`preset default: access: unknown key "central"`,
		`unknown key "profile"`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings lack %q:\n%s", want, joined)
		}
	}
}

// A startup or access group present but empty is not an error: its settings
// take their defaults.
func TestLoadAcceptsEmptyGroups(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, FileName), "presets:\n  default:\n    startup:\n    access:\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	def := preset(t, cfg, DefaultPreset)
	if def.LocalStore != LocalAll || !def.CentralStore || len(def.Profiles) != 0 {
		t.Errorf("default = %+v, want no profiles with the default settings", def)
	}
	if len(cfg.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", cfg.Warnings)
	}
}

func TestLoadMissingConfigIsZero(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Presets) != 0 || cfg.Path != "" || cfg.Migrated {
		t.Errorf("expected a zero Config, got %+v", cfg)
	}
}

// Select falls back to default for an empty name, and to a blank preset with
// the default settings when nothing matches — never a zero Preset, whose
// Central would read as false.
func TestSelect(t *testing.T) {
	cfg := Config{Presets: []Preset{
		{Name: "default", Profiles: []string{"general"}, LocalStore: LocalAll, CentralStore: true},
		{Name: "frontend", Profiles: []string{"react"}, LocalStore: LocalSelected, CentralStore: false},
	}}

	if p, ok := cfg.Select(""); !ok || p.Name != "default" {
		t.Errorf(`Select("") = %q, %v; want default`, p.Name, ok)
	}
	if p, ok := cfg.Select("frontend"); !ok || p.Name != "frontend" {
		t.Errorf(`Select("frontend") = %q, %v`, p.Name, ok)
	}
	p, ok := Config{}.Select("")
	if ok {
		t.Error("Select reported finding a preset in an empty config")
	}
	if p.LocalStore != LocalAll || !p.CentralStore || len(p.Profiles) != 0 {
		t.Errorf("blank preset = %+v, want no profiles with the default settings", p)
	}
}

// --- writing -----------------------------------------------------------------

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
	if cfg.Migrated {
		t.Error("a freshly written config was treated as needing migration")
	}
	def := preset(t, cfg, DefaultPreset)
	if want := []string{"python"}; !reflect.DeepEqual(def.Profiles, want) {
		t.Errorf("Bootstrap = %v, want %v", def.Profiles, want)
	}
	if len(def.Files) != 1 || def.Files[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles = %v, want [AGENTS.md]", def.Files)
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
		if got := preset(t, cfg, DefaultPreset).Profiles; !reflect.DeepEqual(got, want) {
			t.Errorf("round trip of %v gave %v", want, got)
		}
	}
}

// A generated config teaches local_store and central_store — commented out, so
// their defaults stay in force — and every comment survives into the file in
// both places a config can live. yaml.v3 drops some trailing comments, and a
// group holding nothing but comments is the most exposed: hence checking the
// access block and the example of a second preset in particular.
func TestWriteDocumentsTheSettings(t *testing.T) {
	for _, tc := range []struct{ name, file, seed string }{
		{"own file", FileName, ""},
		{"go.yaml", GoYAMLFile, goYAML},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.seed != "" {
				write(t, filepath.Join(dir, tc.file), tc.seed)
			}
			path, err := Write(dir, []string{"python"}, nil)
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			got := readFile(t, path)
			for _, want := range []string{
				"# startup: what a session with this preset starts with.",
				"profiles: [python]",
				"# local_store: all", "selected   only those the profiles above select by tag",
				"# access: what a session with this preset may reach at all.",
				"#   central_store: true", "false      this project's local store only",
				"#     frontend:", "#         profiles: [react]", // the example of a second preset
				"keep default", "light",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("%s lacks %q:\n%s", tc.file, want, got)
				}
			}

			cfg, err := Load(dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			// Commented out means the defaults, and the example is not a preset.
			if want := []string{"default"}; !reflect.DeepEqual(cfg.Names(), want) {
				t.Errorf("Names = %v, want %v", cfg.Names(), want)
			}
			def := preset(t, cfg, DefaultPreset)
			if def.LocalStore != LocalAll || !def.CentralStore {
				t.Errorf("settings = local %q central %v, want the defaults", def.LocalStore, def.CentralStore)
			}
		})
	}
}

// Where a comment sits matters as much as whether it survives: one that drifts
// into the files list reads as if it belonged to it. yaml.v3 re-indents a
// comment that follows a block list, so this writes a non-empty files list —
// the case that moved it — and checks each setting's documentation stays level
// with the keys it sits among, in both places a config can live.
func TestWriteKeepsCommentsAtTheirKeys(t *testing.T) {
	for _, tc := range []struct {
		name, file, seed string
		indent           string // the preset's own indentation in this file
	}{
		{"own file", FileName, "", "    "},
		{"go.yaml", GoYAMLFile, goYAML, "        "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.seed != "" {
				write(t, filepath.Join(dir, tc.file), tc.seed)
			}
			path, err := Write(dir, []string{"python"}, []string{"AGENTS.md", "docs/arch.md"})
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			got := readFile(t, path)
			inStartup := tc.indent + "  "
			for _, want := range []string{
				"\n" + tc.indent + "startup:\n",
				"\n" + inStartup + "# local_store: all\n",
				"\n" + inStartup + "files:\n",
				"\n" + tc.indent + "# access:\n",
				"\n" + tc.indent + "#   central_store: true\n",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("%s lacks %q at its key's indentation:\n%s", tc.file, strings.TrimPrefix(want, "\n"), got)
				}
			}
		})
	}
}

// --- host manifests ------------------------------------------------------------

// A project that keeps a go.yaml can hold contour's settings in it.
func TestLoadReadsGoYAMLExternalSection(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML+
		"  contour:\n    presets:\n      default:\n        startup:\n          profiles: [python, cli]\n          files:\n            - AGENTS.md\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	def := preset(t, cfg, DefaultPreset)
	if want := []string{"python", "cli"}; !reflect.DeepEqual(def.Profiles, want) {
		t.Errorf("Bootstrap = %v, want %v", def.Profiles, want)
	}
	if len(def.Files) != 1 || def.Files[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles = %v, want [AGENTS.md]", def.Files)
	}
	if cfg.Path != filepath.Join(dir, GoYAMLFile) {
		t.Errorf("Path = %q, want the go.yaml", cfg.Path)
	}
}

// A project keeps its go.yaml for reasons of its own. One without a contour
// section must not be mistaken for a contour config, nor disturb the
// .contour.yaml beside it.
func TestLoadIgnoresGoYAMLWithoutContourSection(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML)
	write(t, filepath.Join(dir, FileName), "presets:\n  default:\n    startup:\n      profiles: [python]\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"python"}; !reflect.DeepEqual(preset(t, cfg, DefaultPreset).Profiles, want) {
		t.Errorf("Profiles = %v, want %v from .contour.yaml", preset(t, cfg, DefaultPreset).Profiles, want)
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
	write(t, filepath.Join(dir, GoYAMLFile), goYAML+"  contour:\n    presets:\n      default:\n        startup:\n          profiles: [fromgo]\n")
	write(t, filepath.Join(dir, FileName), "presets:\n  default:\n    startup:\n      profiles: [fromown]\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"fromown"}; !reflect.DeepEqual(preset(t, cfg, DefaultPreset).Profiles, want) {
		t.Errorf("Bootstrap = %v, want %v (.contour.yaml wins)", preset(t, cfg, DefaultPreset).Profiles, want)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], GoYAMLFile) {
		t.Errorf("expected a warning naming %s, got %v", GoYAMLFile, cfg.Warnings)
	}
}

// A go.yaml contour cannot parse belongs to the project, not to contour.
// It must degrade to a warning, not break every command.
func TestLoadToleratesUnreadableGoYAML(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), "name: [unclosed\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an error for a malformed go.yaml: %v", err)
	}
	if len(cfg.Presets) != 0 {
		t.Errorf("Presets = %v, want none", cfg.Names())
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
	def := preset(t, cfg, DefaultPreset)
	if want := []string{"python", "cli"}; !reflect.DeepEqual(def.Profiles, want) {
		t.Errorf("round trip gave %v, want %v", def.Profiles, want)
	}
	if len(def.Files) != 1 || def.Files[0].ID != "AGENTS.md" {
		t.Errorf("round trip gave EagerFiles %v", def.Files)
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
	got := readFile(t, path)
	for _, want := range []string{
		"name: myproject", "description: a thing", "version: 1.2.3",
		"gomore:", "build: \"go build ./...\"", "contour:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("go.yaml lost %q after writing contour's section:\n%s", want, got)
		}
	}
	if strings.Index(got, "external:") > strings.Index(got, "contour:") {
		t.Errorf("contour's section is not under external:\n%s", got)
	}
}

// Overwriting an existing contour section must replace it, not append a second.
func TestWriteReplacesAnExistingContourSection(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML+"  contour:\n    presets:\n      default:\n        startup:\n          profiles: [old]\n")

	if _, err := Write(dir, []string{"new"}, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := readFile(t, filepath.Join(dir, GoYAMLFile))
	if n := strings.Count(got, "\n  contour:"); n != 1 {
		t.Errorf("go.yaml has %d contour sections, want 1:\n%s", n, got)
	}
	if strings.Contains(got, "old") {
		t.Errorf("the previous bootstrap survived:\n%s", got)
	}
}

// An existing .contour.yaml is what Load reads, so it is what Write must update.
// Writing into the host manifest instead would file the settings somewhere
// permanently shadowed.
func TestWritePrefersAnExistingOwnFileOverAHost(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, GoYAMLFile), goYAML)
	write(t, filepath.Join(dir, FileName), "presets:\n  default:\n    startup:\n      profiles: [old]\n")

	path, err := Write(dir, []string{"new"}, nil)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if path != filepath.Join(dir, FileName) {
		t.Errorf("Write path = %q, want the existing .contour.yaml", path)
	}
	if strings.Contains(readFile(t, filepath.Join(dir, GoYAMLFile)), "contour:") {
		t.Error("Write added a shadowed section to go.yaml")
	}
}

// --- migration -------------------------------------------------------------------

// The pre-preset shape becomes the default preset — every setting carried over —
// and the file is rewritten in the new shape, once.
func TestLoadMigratesTheOldShapeIntoTheDefaultPreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	write(t, path, "bootstrap: [go, cli]\neager_files:\n  - AGENTS.md\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Migrated {
		t.Error("Migrated = false for an old-shape config")
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "migrated") {
		t.Errorf("expected one migration notice, got %v", cfg.Warnings)
	}
	def := preset(t, cfg, DefaultPreset)
	if want := []string{"go", "cli"}; !reflect.DeepEqual(def.Profiles, want) {
		t.Errorf("Bootstrap = %v, want %v carried over", def.Profiles, want)
	}
	if len(def.Files) != 1 || def.Files[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles = %v, want [AGENTS.md] carried over", def.Files)
	}

	// The file itself is now in the new shape, documented like a generated one.
	got := readFile(t, path)
	for _, want := range []string{"presets:", "  default:", "    startup:", "profiles: [go, cli]", "- AGENTS.md", "# local_store: all", "#   central_store: true"} {
		if !strings.Contains(got, want) {
			t.Errorf("migrated file lacks %q:\n%s", want, got)
		}
	}

	// And it migrates exactly once: the next load reads it as it stands.
	again, err := Load(dir)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if again.Migrated || len(again.Warnings) != 0 {
		t.Errorf("second Load migrated again (%v, %v)", again.Migrated, again.Warnings)
	}
	unchanged(t, path, got)
}

// A legacy section inside go.yaml migrates in place, leaving the rest of the
// file's settings alone.
func TestLoadMigratesAGoYAMLSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, GoYAMLFile)
	write(t, path, goYAML+"  contour:\n    bootstrap: [go]\n    eager_files: []\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Migrated {
		t.Fatal("Migrated = false for an old-shape go.yaml section")
	}
	got := readFile(t, path)
	for _, want := range []string{"name: myproject", "version: 1.2.3", "build: \"go build ./...\"", "presets:", "startup:", "profiles: [go]"} {
		if !strings.Contains(got, want) {
			t.Errorf("migrated go.yaml lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\n    bootstrap: [go]\n    eager_files") {
		t.Errorf("the old top-level keys are still in the contour section:\n%s", got)
	}
	if want := []string{"go"}; !reflect.DeepEqual(preset(t, cfg, DefaultPreset).Profiles, want) {
		t.Errorf("Bootstrap = %v, want %v", preset(t, cfg, DefaultPreset).Profiles, want)
	}
}

// An unwritable config must not stop a session: it is used, migrated in
// memory, and the user is told the file could not be updated.
func TestLoadServesAnOldConfigItCannotRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	body := "bootstrap: [go]\n"
	write(t, path, body)
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migrated {
		t.Error("Migrated = true although the file could not be written")
	}
	if want := []string{"go"}; !reflect.DeepEqual(preset(t, cfg, DefaultPreset).Profiles, want) {
		t.Errorf("Bootstrap = %v, want %v from the in-memory migration", preset(t, cfg, DefaultPreset).Profiles, want)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "could not be migrated") {
		t.Errorf("expected a could-not-migrate warning, got %v", cfg.Warnings)
	}
	unchanged(t, path, body)
}

// A config with both shapes is ambiguous; guessing which wins would silently
// drop settings either way.
func TestLoadRejectsAMixOfOldAndNewShapes(t *testing.T) {
	dir := t.TempDir()
	body := "bootstrap: [go]\npresets:\n  default:\n    startup:\n      profiles: [python]\n"
	write(t, filepath.Join(dir, FileName), body)

	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "presets.default") {
		t.Errorf("err = %v, want one pointing at presets.default", err)
	}
	unchanged(t, filepath.Join(dir, FileName), body)
}

// --- Ensure ------------------------------------------------------------------------

func TestEnsureCreatesAConfigWhenNoneExists(t *testing.T) {
	dir := t.TempDir()

	cfg, created, err := Ensure(dir, []string{"python", "cli"}, []string{"AGENTS.md"})
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !created {
		t.Error("created = false for a project with no config")
	}
	if cfg.Path != filepath.Join(dir, FileName) {
		t.Errorf("Path = %q, want the new .contour.yaml", cfg.Path)
	}
	def := preset(t, cfg, DefaultPreset)
	if want := []string{"python", "cli"}; !reflect.DeepEqual(def.Profiles, want) {
		t.Errorf("Bootstrap = %v, want %v", def.Profiles, want)
	}
	if len(def.Files) != 1 || def.Files[0].ID != "AGENTS.md" {
		t.Errorf("EagerFiles = %v, want [AGENTS.md]", def.Files)
	}
}

// Every developer re-runs setup on their own machine. With no profiles asked
// for, an existing config — in either place it can live — must come through
// untouched, including eager files that setup would otherwise have re-detected.
func TestEnsureLeavesAnExistingConfigUntouched(t *testing.T) {
	for _, tc := range []struct {
		name, file, body string
	}{
		{"own file", FileName,
			"# the team's notes\npresets:\n  default:\n    startup:\n      profiles: [go]   # chosen on purpose\n\n      files:\n        - NOTES.md\n"},
		{"go.yaml section", GoYAMLFile,
			goYAML + "  contour:\n    # the team's notes\n    presets:\n      default:\n        startup:\n          profiles: [go]\n          files: [NOTES.md]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.file)
			write(t, path, tc.body)

			cfg, created, err := Ensure(dir, nil, []string{"AGENTS.md"})
			if err != nil {
				t.Fatalf("Ensure: %v", err)
			}
			if created {
				t.Error("created = true for a project that already had a config")
			}
			unchanged(t, path, tc.body)
			def := preset(t, cfg, DefaultPreset)
			if want := []string{"go"}; !reflect.DeepEqual(def.Profiles, want) {
				t.Errorf("Bootstrap = %v, want the existing %v", def.Profiles, want)
			}
			if len(def.Files) != 1 || def.Files[0].ID != "NOTES.md" {
				t.Errorf("EagerFiles = %v, want the existing [NOTES.md], not re-detected ones", def.Files)
			}
		})
	}
}

// Re-running the very command that created the config must be harmless.
func TestEnsureAcceptsTheProfilesAlreadySelected(t *testing.T) {
	dir := t.TempDir()
	body := "presets:\n  default:\n    startup:\n      profiles: [python, cli]\n"
	write(t, filepath.Join(dir, FileName), body)

	// Duplicates and stray whitespace normalise to the same selection.
	if _, created, err := Ensure(dir, []string{"python", " cli", "python"}, nil); err != nil || created {
		t.Fatalf("Ensure = (created %v, err %v), want an untouched no-op", created, err)
	}
	unchanged(t, filepath.Join(dir, FileName), body)
}

// Different profiles against an existing default preset are refused, with
// nothing written. Order counts: it decides the order of the composed payload.
func TestEnsureRefusesToChangeTheProfiles(t *testing.T) {
	for _, tc := range []struct {
		name      string
		existing  string
		requested []string
	}{
		{"different profile", "presets:\n  default:\n    startup:\n      profiles: [go]\n", []string{"python"}},
		{"different order", "presets:\n  default:\n    startup:\n      profiles: [python, cli]\n", []string{"cli", "python"}},
		{"existing is empty", "presets:\n  default:\n    startup:\n      profiles: []\n", []string{"python"}},
		{"no default preset", "presets:\n  frontend:\n    startup:\n      profiles: [react]\n", []string{"python"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, FileName)
			write(t, path, tc.existing)

			cfg, created, err := Ensure(dir, tc.requested, nil)
			if !errors.Is(err, ErrProfileConflict) {
				t.Fatalf("err = %v, want ErrProfileConflict", err)
			}
			if created {
				t.Error("created = true on a conflict")
			}
			if cfg.Path != path {
				t.Errorf("conflict returned Path %q, want the existing %q", cfg.Path, path)
			}
			unchanged(t, path, tc.existing)
		})
	}
}

// A config contour cannot parse must not be "repaired" by overwriting it.
func TestEnsureDoesNotOverwriteAMalformedConfig(t *testing.T) {
	dir := t.TempDir()
	body := "presets: [unclosed\n"
	write(t, filepath.Join(dir, FileName), body)

	if _, _, err := Ensure(dir, []string{"python"}, nil); err == nil {
		t.Error("Ensure succeeded over a malformed .contour.yaml")
	}
	unchanged(t, filepath.Join(dir, FileName), body)
}

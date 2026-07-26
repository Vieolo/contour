// Package project reads a project's contour config from an overlay folder in the
// working directory: which central bootstrap profile to load, and which single
// files to load eagerly. It is the per-project counterpart to the machine-wide
// config in internal/config.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vieolo/contour/internal/store"
	"gopkg.in/yaml.v3"
)

// ConfigFileName is the config file contour looks for inside an overlay folder.
const ConfigFileName = "config.yaml"

// Config is a project's contour config, read from the first recognised overlay
// folder that holds a config file.
type Config struct {
	// Bootstrap is the central profile to load eagerly, unless the --bootstrap
	// flag overrides it.
	Bootstrap string

	// EagerFiles are files to load eagerly as local rules, resolved to absolute
	// paths.
	EagerFiles []store.EagerFile

	// Path is the config file that was read, or empty if none was found.
	Path string
}

type file struct {
	Bootstrap  string   `yaml:"bootstrap"`
	EagerFiles []string `yaml:"eager_files"`
}

// Load reads the project config from the first recognised overlay folder under
// projectDir (in store.OverlayDirNames order) that contains a config file. A
// missing config is not an error; it returns a zero Config.
func Load(projectDir string) (Config, error) {
	for _, name := range store.OverlayDirNames {
		path := filepath.Join(projectDir, name, ConfigFileName)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, fmt.Errorf("read %s: %w", path, err)
		}

		var f file
		if err := yaml.Unmarshal(data, &f); err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", path, err)
		}
		return Config{
			Bootstrap:  strings.TrimSpace(f.Bootstrap),
			EagerFiles: resolveEagerFiles(projectDir, f.EagerFiles),
			Path:       path,
		}, nil
	}
	return Config{}, nil
}

// Write creates or overwrites the project config with a bootstrap profile and a
// list of eager files. It is placed in the first existing overlay folder under
// projectDir, or .contour/ if none exists yet, and returns the path written.
// The file is rendered from a template so it documents its own fields.
func Write(projectDir, bootstrap string, eagerFiles []string) (string, error) {
	dir := writeDir(projectDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, render(bootstrap, eagerFiles), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// writeDir picks where a new project config goes: an overlay folder the project
// already uses, or .contour/ as the default.
func writeDir(projectDir string) string {
	for _, name := range store.OverlayDirNames {
		if info, err := os.Stat(filepath.Join(projectDir, name)); err == nil && info.IsDir() {
			return filepath.Join(projectDir, name)
		}
	}
	return filepath.Join(projectDir, store.OverlayDirNames[0])
}

func render(bootstrap string, eagerFiles []string) []byte {
	var b strings.Builder

	b.WriteString("# contour project config\n")
	b.WriteString("#\n")
	b.WriteString("# Per-project settings, read from the first of .contour/, .agents/ or\n")
	b.WriteString("# .claude/ that holds this file. Commit it so your team shares the setup.\n")
	b.WriteString("\n")
	b.WriteString("# bootstrap: the central store profile whose rules load eagerly here.\n")
	b.WriteString("#   The --bootstrap flag to `contour mcp` overrides it.\n")
	fmt.Fprintf(&b, "bootstrap: %q\n", bootstrap)
	b.WriteString("\n")
	b.WriteString("# eager_files: files loaded eagerly as project rules, paths relative to the\n")
	b.WriteString("#   project root. Keep a CLAUDE.md or AGENTS.md without moving it into rules/.\n")
	if len(eagerFiles) == 0 {
		b.WriteString("eager_files: []\n")
	} else {
		b.WriteString("eager_files:\n")
		for _, f := range eagerFiles {
			fmt.Fprintf(&b, "  - %s\n", f)
		}
	}
	return []byte(b.String())
}

// resolveEagerFiles turns listed paths (relative to the project root) into
// EagerFiles: the listed path is the ID, the absolute path is what to read.
func resolveEagerFiles(projectDir string, listed []string) []store.EagerFile {
	var out []store.EagerFile
	for _, p := range listed {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs := p
		if !filepath.IsAbs(p) {
			abs = filepath.Join(projectDir, filepath.FromSlash(p))
		}
		out = append(out, store.EagerFile{ID: filepath.ToSlash(p), Path: abs})
	}
	return out
}

// Package project reads a project's contour config from a .contour.yaml at the
// project root: which central bootstrap profiles to load, and which single files
// to load eagerly.
//
// It lives at the project root, not inside an overlay folder, for the same
// reason the machine-wide config lives outside the store: settings must not
// travel with the content they configure. A user may move their local rules from
// .contour/ to .agents/ for broader tool support; the config stays put and keeps
// working.
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

// FileName is the project config file, at the project root.
const FileName = ".contour.yaml"

// Config is a project's contour config.
type Config struct {
	// Bootstrap lists the central profiles to load eagerly, in the order they
	// should compose, unless the --bootstrap flag overrides them.
	Bootstrap []string

	// EagerFiles are files to load eagerly as local rules, resolved to absolute
	// paths.
	EagerFiles []store.EagerFile

	// Path is the config file that was read, or empty if none was found.
	Path string
}

type file struct {
	Bootstrap  profileNames `yaml:"bootstrap"`
	EagerFiles []string     `yaml:"eager_files"`
}

// profileNames accepts the bootstrap key as either a single name or a list of
// them. Both shapes stay valid so a `bootstrap: python` written before profiles
// could compose keeps working untouched, and so the common single-profile case
// need not be a one-element list.
type profileNames []string

func (n *profileNames) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var one string
		if err := node.Decode(&one); err != nil {
			return err
		}
		*n = profileNames{one}
	case yaml.SequenceNode:
		var many []string
		if err := node.Decode(&many); err != nil {
			return err
		}
		*n = many
	default:
		return fmt.Errorf("bootstrap must be a profile name or a list of profile names")
	}
	return nil
}

// cleanNames trims the listed names and drops blanks and repeats, keeping the
// first occurrence's position — the order decides the order of the composed
// payload.
func cleanNames(names []string) []string {
	seen := make(map[string]bool)

	var out []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// Load reads the project config from .contour.yaml at projectDir. A missing
// config is not an error; it returns a zero Config.
func Load(projectDir string) (Config, error) {
	path := filepath.Join(projectDir, FileName)

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return Config{
		Bootstrap:  cleanNames(f.Bootstrap),
		EagerFiles: resolveEagerFiles(projectDir, f.EagerFiles),
		Path:       path,
	}, nil
}

// Write creates or overwrites the project config at projectDir with a list of
// bootstrap profiles and a list of eager files, returning the path written. The
// file is rendered from a template so it documents its own fields.
func Write(projectDir string, bootstrap []string, eagerFiles []string) (string, error) {
	path := filepath.Join(projectDir, FileName)
	if err := os.WriteFile(path, render(bootstrap, eagerFiles), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

func render(bootstrap []string, eagerFiles []string) []byte {
	var b strings.Builder

	b.WriteString("# contour project config\n")
	b.WriteString("#\n")
	b.WriteString("# Per-project settings, at the project root so they are independent of which\n")
	b.WriteString("# overlay folder (.contour/, .agents/ or .claude/) holds your local rules —\n")
	b.WriteString("# move those between folders freely without losing this. Commit it so your\n")
	b.WriteString("# team shares the setup.\n")
	b.WriteString("\n")
	b.WriteString("# bootstrap: the central store profiles whose rules load eagerly here.\n")
	b.WriteString("#   List more than one to combine entry points — [python, cli] for a Python\n")
	b.WriteString("#   project that also ships a CLI. They compose in order, and an item both\n")
	b.WriteString("#   profiles select is loaded once. A bare name works too: bootstrap: python\n")
	b.WriteString("#   The --bootstrap flag to `contour mcp` overrides this.\n")
	fmt.Fprintf(&b, "bootstrap: [%s]\n", strings.Join(cleanNames(bootstrap), ", "))
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

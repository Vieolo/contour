// Package project reads a project's contour config — which central bootstrap
// profiles to load, and which single files to load eagerly.
//
// The config lives at the project root, not inside an overlay folder, for the
// same reason the machine-wide config lives outside the store: settings must not
// travel with the content they configure. A user may move their local rules from
// .contour/ to .agents/ for broader tool support; the config stays put and keeps
// working.
//
// It can be held in one of two places. Its own .contour.yaml always works. But a
// project that already keeps a manifest for its ecosystem — go.yaml today,
// pyproject.toml or package.json in time — can carry contour's settings in a
// section of that file instead, sparing itself another dotfile.
//
// .contour.yaml wins wherever both exist. Being contour's own file makes it the
// unambiguous statement of intent, and the rule stays a single comparison however
// many host manifests are supported: were a host to win instead, every new one
// would have to be ranked against all the others.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/godotyaml"
	"gopkg.in/yaml.v3"
)

const (
	// FileName is contour's own project config file, at the project root. It
	// takes precedence over any host manifest.
	FileName = ".contour.yaml"

	// GoYAMLFile is the go.yaml a Go project may already keep for its metadata
	// and tool configuration.
	GoYAMLFile = "go.yaml"

	// ExternalKey is contour's section within go.yaml's `external` mapping. The
	// go.yaml spec reserves the root for its own schema and gives each tool a
	// named section under `external`, so contour's settings live at
	// external.contour and cannot collide with another tool's.
	ExternalKey = "contour"
)

// host is a project manifest that can carry contour's settings in a section of
// its own, so a project need not add a dedicated config file for them.
//
// read reports whether the file holds a contour section at all — a project keeps
// its manifest for reasons of its own, and a bare one must not be mistaken for a
// contour config. write assumes the file exists; both are given its full path.
type host struct {
	Name    string
	Section string // where the settings live, for messages
	read    func(path string) (file, bool, error)
	write   func(path string, bootstrap, eagerFiles []string) error
}

// hosts are the manifests contour can read its settings from, in the order they
// are tried. Only go.yaml is supported today; pyproject.toml and package.json
// are the obvious next entries, and adding one means appending here rather than
// touching Load or Write.
var hosts = []host{{
	Name:    GoYAMLFile,
	Section: "external." + ExternalKey,
	read:    readGoYAML,
	write:   writeGoYAML,
}}

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

	// Warnings describe config that was found but not used — a second config
	// file shadowed by the one in effect, or a go.yaml that could not be read.
	// They are reported to the user rather than returned as errors: contour
	// still has a usable config, and refusing to run because an unrelated file
	// is malformed would be worse than saying so and carrying on.
	Warnings []string
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

// Load reads the project config: .contour.yaml if the project has one, else the
// contour section of a host manifest. A missing config is not an error; it
// returns a zero Config.
//
// A host is consulted only when it actually holds a contour section. A project
// keeps its manifest for reasons of its own, so a bare go.yaml must never be
// read as a contour config — and adding one to a project must never disturb the
// .contour.yaml already sitting beside it.
func Load(projectDir string) (Config, error) {
	ownPath := filepath.Join(projectDir, FileName)
	ownFile, ownFound, err := loadFromOwnFile(ownPath)
	if err != nil {
		return Config{}, err
	}

	hostFile, hostPath, hostFound, warnings := loadFromHosts(projectDir)

	switch {
	case ownFound:
		// The dedicated file wins, but a host section that will now never be read
		// is worth naming: silently ignoring settings someone wrote is how a
		// project ends up mystified about which ones are in force.
		if hostFound {
			warnings = append(warnings, fmt.Sprintf(
				"%s is in effect, so contour's settings in %s are ignored", ownPath, hostPath))
		}
		return configFrom(projectDir, ownFile, ownPath, warnings), nil
	case hostFound:
		return configFrom(projectDir, hostFile, hostPath, warnings), nil
	default:
		return Config{Warnings: warnings}, nil
	}
}

// loadFromHosts returns the first host manifest carrying a contour section.
func loadFromHosts(projectDir string) (f file, path string, found bool, warnings []string) {
	for _, h := range hosts {
		p := filepath.Join(projectDir, h.Name)
		if _, err := os.Stat(p); err != nil {
			continue
		}

		hf, ok, err := h.read(p)
		if err != nil {
			// The manifest belongs to the project, not to contour. One contour
			// cannot read degrades to a warning rather than an error: a project
			// must not become unusable because a file contour merely borrows is
			// malformed.
			warnings = append(warnings, fmt.Sprintf("could not read contour's settings from %s (%v)", p, err))
			continue
		}
		if ok {
			return hf, p, true, warnings
		}
	}
	return file{}, "", false, warnings
}

func configFrom(projectDir string, f file, path string, warnings []string) Config {
	return Config{
		Bootstrap:  cleanNames(f.Bootstrap),
		EagerFiles: resolveEagerFiles(projectDir, f.EagerFiles),
		Path:       path,
		Warnings:   warnings,
	}
}

// loadFromOwnFile reads .contour.yaml, reporting whether it exists.
func loadFromOwnFile(path string) (file, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return file{}, false, nil
	}
	if err != nil {
		return file{}, false, fmt.Errorf("read %s: %w", path, err)
	}

	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return file{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, true, nil
}

// readGoYAML reads external.contour from a go.yaml, reporting whether that
// section is present.
func readGoYAML(path string) (f file, found bool, err error) {
	doc, err := godotyaml.Load(path)
	if err != nil {
		return file{}, false, err
	}
	found, err = doc.DecodeExternalConfig(ExternalKey, &f)
	if err != nil {
		return file{}, false, err
	}
	return f, found, nil
}

// Write records the bootstrap profiles and eager files for a project, returning
// the path written.
//
// A project that already keeps a host manifest gets contour's settings in that
// file rather than a second config file. One that does not gets a .contour.yaml.
// Either way the result documents its own fields.
//
// An existing .contour.yaml is always the target, because it is what Load will
// read: writing to a host manifest while one sits beside it would file the
// settings somewhere permanently shadowed.
func Write(projectDir string, bootstrap []string, eagerFiles []string) (string, error) {
	ownPath := filepath.Join(projectDir, FileName)

	if _, err := os.Stat(ownPath); err != nil {
		for _, h := range hosts {
			p := filepath.Join(projectDir, h.Name)
			if _, err := os.Stat(p); err != nil {
				continue
			}
			return p, h.write(p, bootstrap, eagerFiles)
		}
	}

	if err := os.WriteFile(ownPath, render(bootstrap, eagerFiles), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", ownPath, err)
	}
	return ownPath, nil
}

// writeGoYAML sets external.contour in an existing go.yaml, leaving the rest of
// the document — including its comments, key order and any other tool's section
// — exactly as it was.
func writeGoYAML(path string, bootstrap, eagerFiles []string) error {
	doc, err := godotyaml.Load(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := doc.SetExternalConfig(ExternalKey, externalNode(bootstrap, eagerFiles)); err != nil {
		return fmt.Errorf("set external.%s in %s: %w", ExternalKey, path, err)
	}
	if err := doc.Save(path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// externalNode builds the external.contour mapping with its documentation
// attached as comments, so the section explains itself in go.yaml just as
// .contour.yaml does. It is passed as a node rather than a plain struct
// precisely because a struct would marshal without the comments.
func externalNode(bootstrap, eagerFiles []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	bootstrapKey := scalar("bootstrap")
	bootstrapKey.HeadComment = "the central store profiles whose rules load eagerly here.\n" +
		"List more than one to combine entry points — [python, cli] for a Python\n" +
		"project that also ships a CLI. They compose in order, and an item both\n" +
		"profiles select is loaded once. A bare name works too: bootstrap: python\n" +
		"The --bootstrap flag to `contour mcp` overrides this."
	names := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, name := range cleanNames(bootstrap) {
		names.Content = append(names.Content, scalar(name))
	}

	eagerKey := scalar("eager_files")
	eagerKey.HeadComment = "files loaded eagerly as project rules, paths relative to the project\n" +
		"root. Keep a CLAUDE.md or AGENTS.md without moving it into rules/."
	files := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	if len(eagerFiles) == 0 {
		files.Style = yaml.FlowStyle
	}
	for _, f := range eagerFiles {
		files.Content = append(files.Content, scalar(f))
	}

	node.Content = append(node.Content, bootstrapKey, names, eagerKey, files)
	return node
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
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

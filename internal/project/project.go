// Package project reads a project's contour config: its presets — named
// starting points for an agent session, each choosing the central profiles and
// project files to load eagerly.
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
	"slices"
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

	// DefaultPreset is the preset a session starts with when none is named. It is
	// the only reserved preset name.
	DefaultPreset = "default"
)

// LocalMode says which items from the project's local store a preset includes.
type LocalMode string

const (
	// LocalAll includes every local item: local rules load eagerly, and local
	// skills and knowledge join the menu. Being local is scope enough.
	LocalAll LocalMode = "all"

	// LocalSelected includes only the local items the preset's profiles select
	// by tag, as central items are; the rest stay reachable through list,
	// search and get.
	LocalSelected LocalMode = "selected"
)

// Preset is a named starting point for an agent session in this project.
type Preset struct {
	Name        string
	Description string

	// Bootstrap lists the profiles to load eagerly, in the order they compose.
	Bootstrap []string

	// EagerFiles are project files loaded eagerly as local rules, resolved to
	// absolute paths.
	EagerFiles []store.EagerFile

	// Local says which local items the preset includes. Never empty once loaded.
	Local LocalMode

	// Central reports whether the preset uses the central store at all.
	Central bool
}

// Config is a project's contour config.
type Config struct {
	// Presets are the project's presets, in the order the file lists them.
	Presets []Preset

	// Path is the config file that was read, or empty if none was found.
	Path string

	// Migrated reports that the file at Path was in the pre-preset shape and
	// this load rewrote it as presets.
	Migrated bool

	// Warnings describe things worth telling the user that do not stop contour
	// from working: a second config file shadowed by the one in effect, a
	// go.yaml that could not be read, a key contour does not recognise, or a
	// config just migrated to presets. They are reported rather than returned as
	// errors, because refusing to run would be worse than saying so.
	Warnings []string
}

// Preset returns the preset with the given name.
func (c Config) Preset(name string) (Preset, bool) {
	for _, p := range c.Presets {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}

// Select returns the preset a session uses: the named one, or default when the
// name is empty, reporting whether it exists. When it does not, the result is a
// blank preset — no profiles, no files, the default settings — which is what a
// project without a config has always had.
func (c Config) Select(name string) (Preset, bool) {
	if name == "" {
		name = DefaultPreset
	}
	if p, ok := c.Preset(name); ok {
		return p, true
	}
	return Preset{Name: name, Local: LocalAll, Central: true}, false
}

// Names returns the presets' names in file order.
func (c Config) Names() []string {
	out := make([]string, 0, len(c.Presets))
	for _, p := range c.Presets {
		out = append(out, p.Name)
	}
	return out
}

// host is a project manifest that can carry contour's settings in a section of
// its own, so a project need not add a dedicated config file for them.
//
// read returns the section's raw node and reports whether the file holds a
// contour section at all — a project keeps its manifest for reasons of its own,
// and a bare one must not be mistaken for a contour config. write replaces the
// section; it assumes the file exists. Both are given the file's full path.
type host struct {
	Name    string
	Section string // where the settings live, for messages
	read    func(path string) (*yaml.Node, bool, error)
	write   func(path string, body *yaml.Node) error
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

// presetFile is the YAML schema of one preset.
type presetFile struct {
	Description string       `yaml:"description"`
	Bootstrap   profileNames `yaml:"bootstrap"`
	EagerFiles  []string     `yaml:"eager_files"`
	Local       string       `yaml:"local"`
	Central     *bool        `yaml:"central"`
}

// Keys contour recognises, so a typo is reported instead of silently ignored —
// `eager_file:` would otherwise leave a preset quietly loading nothing.
var (
	rootKeys   = []string{"presets"}
	legacyKeys = []string{"bootstrap", "eager_files"}
	presetKeys = []string{"description", "bootstrap", "eager_files", "local", "central"}
)

// profileNames accepts the bootstrap key as either a single name or a list of
// them, so the common single-profile case need not be a one-element list.
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
//
// A config still in the pre-preset shape — bootstrap and eager_files at the top
// level — is migrated: its settings become the default preset, and the file is
// rewritten in the new shape, once, with a warning saying so. If the rewrite
// fails the config is still used, migrated in memory, so an unwritable file
// never stops a session.
func Load(projectDir string) (Config, error) {
	ownPath := filepath.Join(projectDir, FileName)
	ownBody, ownFound, err := readOwnFile(ownPath)
	if err != nil {
		return Config{}, err
	}

	hostBody, hostPath, hostWrite, hostFound, warnings := loadFromHosts(projectDir)

	var (
		body  *yaml.Node
		path  string
		write func(*yaml.Node) error
	)
	switch {
	case ownFound:
		// The dedicated file wins, but a host section that will now never be read
		// is worth naming: silently ignoring settings someone wrote is how a
		// project ends up mystified about which ones are in force.
		if hostFound {
			warnings = append(warnings, fmt.Sprintf(
				"%s is in effect, so contour's settings in %s are ignored", ownPath, hostPath))
		}
		body, path = ownBody, ownPath
		write = func(n *yaml.Node) error { return writeOwnFile(ownPath, n) }
	case hostFound:
		body, path = hostBody, hostPath
		write = func(n *yaml.Node) error { return hostWrite(hostPath, n) }
	default:
		return Config{Warnings: warnings}, nil
	}

	presets, legacy, parseWarnings, err := parse(projectDir, body)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	warnings = append(warnings, parseWarnings...)

	migrated := false
	if legacy {
		var msg string
		migrated, msg = migrate(path, presets, write)
		warnings = append(warnings, msg)
	}
	return Config{Presets: presets, Path: path, Migrated: migrated, Warnings: warnings}, nil
}

// migrate rewrites a pre-preset config in the new shape, reporting whether it
// did and the message to show the user either way.
func migrate(path string, presets []Preset, write func(*yaml.Node) error) (bool, string) {
	def := presets[0] // a legacy config migrates to exactly one: the default
	ids := make([]string, 0, len(def.EagerFiles))
	for _, f := range def.EagerFiles {
		ids = append(ids, f.ID)
	}

	body, err := bodyNode(def.Bootstrap, ids)
	if err == nil {
		err = write(body)
	}
	if err != nil {
		return false, fmt.Sprintf("%s uses the old config shape and could not be migrated (%v); "+
			"using its settings as the default preset for now", path, err)
	}
	return true, fmt.Sprintf("migrated %s to presets: its bootstrap and eager_files are now the default preset — "+
		"review the change and commit it", path)
}

// parse reads a config body: a mapping holding presets, or, in a config written
// before presets existed, bootstrap and eager_files at the top level. It reports
// which shape it found.
func parse(projectDir string, body *yaml.Node) (presets []Preset, legacy bool, warnings []string, err error) {
	if body == nil || body.Kind == 0 {
		return nil, false, nil, nil // an empty file
	}
	if body.Kind != yaml.MappingNode {
		return nil, false, nil, errors.New("the config must be a mapping")
	}

	hasLegacy := false
	for _, k := range legacyKeys {
		if mappingValue(body, k) != nil {
			hasLegacy = true
		}
	}
	presetsNode := mappingValue(body, "presets")

	switch {
	case hasLegacy && presetsNode != nil:
		return nil, false, nil, errors.New("it has both presets and the old top-level bootstrap/eager_files; " +
			"move those into presets.default")

	case hasLegacy:
		var p presetFile
		if err := body.Decode(&p); err != nil {
			return nil, false, nil, err
		}
		preset, err := presetFrom(projectDir, DefaultPreset, p)
		if err != nil {
			return nil, false, nil, err
		}
		return []Preset{preset}, true, nil, nil
	}

	warnings = unknownKeys(body, rootKeys, "")
	if presetsNode == nil {
		return nil, false, warnings, nil
	}
	if presetsNode.Kind != yaml.MappingNode {
		return nil, false, nil, errors.New("presets must be a mapping of preset names to presets")
	}

	for i := 0; i+1 < len(presetsNode.Content); i += 2 {
		name := strings.TrimSpace(presetsNode.Content[i].Value)
		value := presetsNode.Content[i+1]
		if name == "" {
			return nil, false, nil, errors.New("a preset has an empty name")
		}
		if value.Kind != yaml.MappingNode {
			return nil, false, nil, fmt.Errorf("preset %q must be a mapping", name)
		}
		warnings = append(warnings, unknownKeys(value, presetKeys, "preset "+name+": ")...)

		var p presetFile
		if err := value.Decode(&p); err != nil {
			return nil, false, nil, fmt.Errorf("preset %q: %w", name, err)
		}
		preset, err := presetFrom(projectDir, name, p)
		if err != nil {
			return nil, false, nil, err
		}
		presets = append(presets, preset)
	}
	return presets, false, warnings, nil
}

func presetFrom(projectDir, name string, p presetFile) (Preset, error) {
	local := LocalMode(strings.TrimSpace(p.Local))
	switch local {
	case "":
		local = LocalAll
	case LocalAll, LocalSelected:
	default:
		return Preset{}, fmt.Errorf("preset %q: local must be %q or %q, not %q", name, LocalAll, LocalSelected, p.Local)
	}

	central := true
	if p.Central != nil {
		central = *p.Central
	}

	return Preset{
		Name:        name,
		Description: strings.TrimSpace(p.Description),
		Bootstrap:   cleanNames(p.Bootstrap),
		EagerFiles:  resolveEagerFiles(projectDir, p.EagerFiles),
		Local:       local,
		Central:     central,
	}, nil
}

// unknownKeys warns about the keys of a mapping contour does not recognise.
func unknownKeys(m *yaml.Node, known []string, prefix string) []string {
	var out []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i].Value; !slices.Contains(known, k) {
			out = append(out, fmt.Sprintf("%sunknown key %q, ignored (expected one of: %s)",
				prefix, k, strings.Join(known, ", ")))
		}
	}
	return out
}

// mappingValue returns the value node for key in mapping m, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// loadFromHosts returns the first host manifest carrying a contour section,
// with the writer for that host.
func loadFromHosts(projectDir string) (body *yaml.Node, path string, write func(string, *yaml.Node) error, found bool, warnings []string) {
	for _, h := range hosts {
		p := filepath.Join(projectDir, h.Name)
		if _, err := os.Stat(p); err != nil {
			continue
		}

		b, ok, err := h.read(p)
		if err != nil {
			// The manifest belongs to the project, not to contour. One contour
			// cannot read degrades to a warning rather than an error: a project
			// must not become unusable because a file contour merely borrows is
			// malformed.
			warnings = append(warnings, fmt.Sprintf("could not read contour's settings from %s (%v)", p, err))
			continue
		}
		if ok {
			return b, p, h.write, true, warnings
		}
	}
	return nil, "", nil, false, warnings
}

// readOwnFile reads .contour.yaml as a node, reporting whether it exists.
func readOwnFile(path string) (*yaml.Node, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{}, true, nil // an empty file
	}
	return doc.Content[0], true, nil
}

// writeOwnFile writes .contour.yaml: the file header, then the body.
func writeOwnFile(path string, body *yaml.Node) error {
	var buf strings.Builder
	buf.WriteString(ownFileHeader)
	// Two-space indentation, as the template is written; yaml.Marshal would
	// re-indent it to four.
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(body); err != nil {
		return fmt.Errorf("render %s: %w", path, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("render %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// readGoYAML reads external.contour from a go.yaml, reporting whether that
// section is present.
func readGoYAML(path string) (*yaml.Node, bool, error) {
	doc, err := godotyaml.Load(path)
	if err != nil {
		return nil, false, err
	}
	node, found := doc.GetRawExternalConfig(ExternalKey)
	return node, found, nil
}

// writeGoYAML sets external.contour in an existing go.yaml, leaving the other
// sections of the document — its metadata and any other tool's settings — as
// they were.
func writeGoYAML(path string, body *yaml.Node) error {
	doc, err := godotyaml.Load(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := doc.SetExternalConfig(ExternalKey, body); err != nil {
		return fmt.Errorf("set external.%s in %s: %w", ExternalKey, path, err)
	}
	if err := doc.Save(path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ErrProfileConflict reports that a project already has a config and its
// default preset selects different profiles from the ones asked for. Ensure
// returns it alongside the existing config, having written nothing.
var ErrProfileConflict = errors.New("the project config already exists and selects different profiles")

// Ensure gives a project a config if it has none, returning the config in
// effect and whether this call created it. A new config holds one preset,
// default, with the given profiles and eager files.
//
// An existing config is never modified. It is the team's committed setup, while
// the command that calls this — registering contour on a machine — is run once
// per developer, on every machine. Rewriting the config there would let anyone's
// routine setup silently replace the team's presets. A deliberate change to
// shared settings should be a deliberate edit to the file, where it shows up in
// review. (Loading an old-shape config migrates it, as any command would; that
// changes its form, not its settings.)
//
// Asking for exactly the profiles the default preset already selects is not a
// conflict, so re-running the same setup command is harmless. Asking for
// different ones returns ErrProfileConflict and writes nothing: quietly ignoring
// an explicit request would be as wrong as quietly honouring it.
//
// eagerFiles are only used when creating; an existing config's list stands.
func Ensure(projectDir string, bootstrap, eagerFiles []string) (Config, bool, error) {
	cfg, err := Load(projectDir)
	if err != nil {
		return Config{}, false, err
	}

	if cfg.Path != "" {
		def, _ := cfg.Preset(DefaultPreset)
		if requested := cleanNames(bootstrap); len(requested) > 0 && !slices.Equal(requested, def.Bootstrap) {
			return cfg, false, ErrProfileConflict
		}
		return cfg, false, nil
	}

	if _, err := Write(projectDir, bootstrap, eagerFiles); err != nil {
		return Config{}, false, err
	}
	// Read back what was written rather than reporting the inputs, so the
	// caller sees the config exactly as every later command will load it.
	cfg, err = Load(projectDir)
	if err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

// Write records a project config holding one preset, default, with the given
// profiles and eager files, returning the path written. It replaces whatever
// config the project had; Ensure is the way to set up a project without that
// risk.
//
// A project that already keeps a host manifest gets contour's settings in that
// file rather than a second config file. One that does not gets a .contour.yaml.
// Either way the result documents its own fields.
//
// An existing .contour.yaml is always the target, because it is what Load will
// read: writing to a host manifest while one sits beside it would file the
// settings somewhere permanently shadowed.
func Write(projectDir string, bootstrap []string, eagerFiles []string) (string, error) {
	body, err := bodyNode(bootstrap, eagerFiles)
	if err != nil {
		return "", err
	}

	ownPath := filepath.Join(projectDir, FileName)
	if _, err := os.Stat(ownPath); err != nil {
		for _, h := range hosts {
			p := filepath.Join(projectDir, h.Name)
			if _, err := os.Stat(p); err != nil {
				continue
			}
			return p, h.write(p, body)
		}
	}

	if err := writeOwnFile(ownPath, body); err != nil {
		return "", err
	}
	return ownPath, nil
}

// bodyNode renders the config body and parses it into a node. The body is
// written once, as commented YAML text, and parsed rather than assembled node by
// node, so .contour.yaml and a host manifest's section share one template and
// document their fields identically.
func bodyNode(bootstrap, eagerFiles []string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(renderBody(bootstrap, eagerFiles)), &doc); err != nil {
		return nil, fmt.Errorf("render the project config: %w", err)
	}
	return doc.Content[0], nil
}

// renderBody renders a config holding a single default preset.
//
// The local and central fields are written commented out, with their values
// explained: a generated config should teach every field it can hold, and
// leaving them commented keeps the defaults in force until someone decides
// otherwise. The example of a second preset sits in the comment above presets:
// yaml.v3 drops a comment trailing the last key of a mapping, so placed there
// it would vanish from go.yaml.
func renderBody(bootstrap, eagerFiles []string) string {
	var b strings.Builder

	b.WriteString(`# presets: named starting points for an agent session in this project. Each
#   one names the profiles and project files that load eagerly. "default" is
#   loaded when a session starts; ask the agent to load any other, or start
#   with it by passing --preset <name> to "contour mcp". A preset inherits
#   nothing from default: each lists everything it needs.
#
#   In a project with several unrelated parts — a monorepo, say — keep default
#   light: it is always loaded, so give it only what every session needs, and
#   put each part in a preset of its own. A second preset looks like this:
#
#     frontend:
#       description: The React app (shown when the agent lists the presets)
#       bootstrap: [react]
#       eager_files: [frontend/AGENTS.md]
presets:
  default:
    # bootstrap: profiles whose rules load eagerly. List several to combine
    #   them — [python, cli] — in the order they compose.
`)
	fmt.Fprintf(&b, "    bootstrap: [%s]\n", strings.Join(cleanNames(bootstrap), ", "))
	b.WriteString(`    # eager_files: project files loaded eagerly as rules, paths relative to the
    #   project root. Keep a CLAUDE.md or AGENTS.md without restructuring it.
`)
	if len(eagerFiles) == 0 {
		b.WriteString("    eager_files: []\n")
	} else {
		b.WriteString("    eager_files:\n")
		for _, f := range eagerFiles {
			fmt.Fprintf(&b, "      - %s\n", f)
		}
	}
	b.WriteString(`    # local: which items from this project's local store (.agents/, .contour/
    #   or .claude/) the preset includes.
    #     all        every local item; local rules load eagerly (the default)
    #     selected   only those the profiles above select by tag; the rest
    #                stay reachable through the list, search and get tools
    # local: all
    # central: whether the preset uses your central store at all.
    #     true       yes (the default)
    #     false      this project's local store only, which keeps a
    #                developer's personal store out of the session
    # central: true
`)
	return b.String()
}

const ownFileHeader = `# contour project config
#
# Per-project settings, at the project root so they are independent of which
# overlay folder (.contour/, .agents/ or .claude/) holds your local rules —
# move those between folders freely without losing this. Commit it so your
# team shares the setup.

`

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

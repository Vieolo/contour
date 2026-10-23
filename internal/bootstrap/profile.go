// Package bootstrap resolves a named bootstrap profile — a store's designed
// entry point — into everything an agent needs to start a session.
//
// A profile selects items by tag. The rules it names are composed eagerly, with
// their full bodies emitted up front, while skills and knowledge are offered as
// a menu the agent fetches on demand. That split is what keeps a session's
// initial context small while leaving the whole store reachable.
//
// Several profiles can be active at once. A project that is mostly Python but
// ships an accessory CLI selects both entry points rather than needing the store
// to carry a combined "python-cli" profile for every such pairing.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vieolo/contour/internal/store"
	"gopkg.in/yaml.v3"
)

// Profile is a bootstrap entry point, parsed from a markdown file in a
// bootstrap/ directory — the central store's, or a project overlay's. Its
// frontmatter holds the tag selections; its body is an optional preamble
// emitted ahead of the rules.
type Profile struct {
	// Name is the profile's filename without the extension (e.g. "python").
	Name string

	// Path is the absolute path to the profile file.
	Path string

	// Source is where the profile was found: the project or the central store.
	Source store.Origin

	// Overrides reports that this is a project profile taking the place of a
	// central one with the same name.
	Overrides bool

	// Description is the profile's one-line summary.
	Description string

	// Preamble is the profile's markdown body, emitted before the eager rules.
	Preamble string

	// RuleTags, SkillTags and KnowledgeTags are the tag selections for each
	// kind. Order matters: it drives the order of the composed output.
	RuleTags      []string
	SkillTags     []string
	KnowledgeTags []string
}

// profileFrontmatter is the YAML schema of a profile's frontmatter block.
type profileFrontmatter struct {
	Description string   `yaml:"description"`
	Rules       []string `yaml:"rules"`
	Skills      []string `yaml:"skills"`
	Knowledge   []string `yaml:"knowledge"`
}

// TagsFor returns the profile's tag selection for a kind, so callers can treat
// the three selections uniformly instead of switching on the field.
func (p Profile) TagsFor(kind store.Kind) []string {
	switch kind {
	case store.KindRules:
		return p.RuleTags
	case store.KindSkills:
		return p.SkillTags
	case store.KindKnowledge:
		return p.KnowledgeTags
	}
	return nil
}

// Dir returns the bootstrap directory inside a store root or overlay.
func Dir(root string) string {
	return filepath.Join(root, store.BootstrapDir)
}

// Sources says where profiles are looked up: the project's overlays first, then
// the central store.
//
// A profile in the project overrides a central one of the same name. That is
// the rule local items already follow — the project's own definition wins —
// and it is what makes a profile portable: one committed with the project means
// the same thing on every machine, whatever each developer's store holds under
// that name.
type Sources struct {
	// Central is the central store's root. Empty leaves the central store out.
	Central string

	// Overlays are the project's overlay directories, in precedence order.
	Overlays []string
}

// CentralOnly looks profiles up in the central store alone.
func CentralOnly(root string) Sources { return Sources{Central: root} }

// find locates a profile by name, reporting where it was found and whether it
// overrides a central profile of the same name.
func (s Sources) find(name string) (path string, origin store.Origin, overrides, found bool, err error) {
	central := ""
	if s.Central != "" {
		central = filepath.Join(Dir(s.Central), name+store.MarkdownExt)
		if ok, err := isFile(central); err != nil {
			return "", "", false, false, err
		} else if !ok {
			central = ""
		}
	}
	for _, ov := range s.Overlays {
		local := filepath.Join(Dir(ov), name+store.MarkdownExt)
		ok, err := isFile(local)
		if err != nil {
			return "", "", false, false, err
		}
		if ok {
			return local, store.OriginLocal, central != "", true, nil
		}
	}
	if central != "" {
		return central, store.OriginStore, false, true, nil
	}
	return "", "", false, false, nil
}

func isFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	return !info.IsDir(), nil
}

// load reads the profile found at path, recording where it came from.
func load(path string, origin store.Origin, overrides bool) (Profile, error) {
	p, err := loadProfile(path)
	if err != nil {
		return Profile{}, err
	}
	p.Source, p.Overrides = origin, overrides
	return p, nil
}

// LoadProfiles reads every profile available — the project's and the central
// store's, a project profile replacing a central one of the same name — ordered
// by name. Missing bootstrap/ directories yield no profiles and no error.
func LoadProfiles(src Sources) ([]Profile, error) {
	byName := make(map[string]Profile)

	if src.Central != "" {
		central, err := readDir(Dir(src.Central), store.OriginStore)
		if err != nil {
			return nil, err
		}
		for _, p := range central {
			byName[p.Name] = p
		}
	}
	// Overlays in reverse precedence, so the first overlay's profile is the one
	// left standing when two define the same name.
	for i := len(src.Overlays) - 1; i >= 0; i-- {
		local, err := readDir(Dir(src.Overlays[i]), store.OriginLocal)
		if err != nil {
			return nil, err
		}
		for _, p := range local {
			if prev, ok := byName[p.Name]; ok {
				p.Overrides = prev.Source == store.OriginStore || prev.Overrides
			}
			byName[p.Name] = p
		}
	}

	profiles := make([]Profile, 0, len(byName))
	for _, p := range byName {
		profiles = append(profiles, p)
	}
	// ReadDir orders by filename, which would put "python-frontend" before
	// "python" ('-' sorts before '.'). Order by profile name instead.
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

// readDir reads the profiles in one bootstrap/ directory.
func readDir(dir string, origin store.Origin) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	var profiles []Profile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != store.MarkdownExt {
			continue
		}
		p, err := load(filepath.Join(dir, e.Name()), origin, false)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// LoadProfile reads a single profile by name — its filename without the
// extension — from the project first, then the central store.
func LoadProfile(src Sources, name string) (Profile, error) {
	path, origin, overrides, found, err := src.find(name)
	if err != nil {
		return Profile{}, err
	}
	if !found {
		return Profile{}, fmt.Errorf("no bootstrap profile named %q", name)
	}
	return load(path, origin, overrides)
}

// LoadNamed reads the named profiles, preserving the order given — that order
// decides the order of the composed payload. It fails on the first name that
// does not exist, rather than quietly composing a partial entry point from the
// rest.
func LoadNamed(src Sources, names []string) ([]Profile, error) {
	profiles := make([]Profile, 0, len(names))
	for _, name := range names {
		p, err := LoadProfile(src, name)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// LoadAvailable reads the named profiles that exist, in the order given, and
// reports the names that do not. Only a profile that exists but cannot be read
// is an error.
//
// It is the lenient counterpart to LoadNamed, for when the names come from a
// project config rather than from the person at the keyboard. A project may
// name profiles that live in each developer's own store, so one developer
// missing one is an ordinary state, not a failure. Treating it as fatal took
// down the whole MCP server, and with it the project's own local rules, which
// have nothing to do with the missing profile. Serving what exists and saying
// what is missing is strictly more useful.
func LoadAvailable(src Sources, names []string) (profiles []Profile, missing []string, err error) {
	for _, name := range names {
		path, origin, overrides, found, err := src.find(name)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			missing = append(missing, name)
			continue
		}
		p, err := load(path, origin, overrides)
		if err != nil {
			return nil, nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, missing, nil
}

func loadProfile(path string) (Profile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read %s: %w", path, err)
	}

	fmText, body, ok := store.SplitFrontmatter(content)
	var fm profileFrontmatter
	if ok {
		if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
			return Profile{}, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	return Profile{
		Name:          strings.TrimSuffix(filepath.Base(path), store.MarkdownExt),
		Path:          path,
		Description:   fm.Description,
		Preamble:      strings.TrimSpace(body),
		RuleTags:      fm.Rules,
		SkillTags:     fm.Skills,
		KnowledgeTags: fm.Knowledge,
	}, nil
}

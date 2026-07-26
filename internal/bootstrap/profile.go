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

// Profile is a bootstrap entry point, parsed from a markdown file in the
// store's bootstrap/ directory. Its frontmatter holds the tag selections; its
// body is an optional preamble emitted ahead of the rules.
type Profile struct {
	// Name is the profile's filename without the extension (e.g. "python").
	Name string

	// Path is the absolute path to the profile file.
	Path string

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

// Dir returns the bootstrap directory inside a store root.
func Dir(root string) string {
	return filepath.Join(root, store.BootstrapDir)
}

// LoadProfiles reads every profile in the store's bootstrap/ directory, ordered
// by name. A missing bootstrap/ directory yields no profiles and no error.
func LoadProfiles(root string) ([]Profile, error) {
	dir := Dir(root)
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
		p, err := loadProfile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}

	// ReadDir orders by filename, which would put "python-frontend" before
	// "python" ('-' sorts before '.'). Order by profile name instead.
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

// LoadProfile reads a single profile by name — its filename without the
// extension.
func LoadProfile(root, name string) (Profile, error) {
	path := filepath.Join(Dir(root), name+store.MarkdownExt)
	if _, err := os.Stat(path); err != nil {
		return Profile{}, fmt.Errorf("no bootstrap profile named %q", name)
	}
	return loadProfile(path)
}

// LoadNamed reads the named profiles, preserving the order given — that order
// decides the order of the composed payload. It fails on the first name that
// does not exist, rather than quietly composing a partial entry point from the
// rest.
func LoadNamed(root string, names []string) ([]Profile, error) {
	profiles := make([]Profile, 0, len(names))
	for _, name := range names {
		p, err := LoadProfile(root, name)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
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

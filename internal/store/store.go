package store

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Store is an in-memory index of the items from a central store and any project
// overlays layered over it. Its exported read methods are safe for concurrent
// use.
type Store struct {
	root  string
	items []Item
	byID  map[string]int
}

// OverlayDirNames are the project-local folders contour recognises in the
// working directory. Any that exist are merged over the central store, so a
// project can carry its own rules alongside the ones people already keep for
// other tools.
var OverlayDirNames = []string{".contour", ".agents", ".claude"}

// EagerFile is a file listed in a project config to be loaded eagerly as a local
// rule — a CLAUDE.md or AGENTS.md a project already keeps, without restructuring
// it into the rules/ layout.
type EagerFile struct {
	ID   string // how it appears, e.g. "AGENTS.md"
	Path string // absolute path to read
}

// Load reads the central store at root.
func Load(root string) (*Store, error) {
	return LoadProject(root, nil, nil)
}

// LoadLayered reads the central store, then merges each overlay over it.
func LoadLayered(central string, overlays ...string) (*Store, error) {
	return LoadProject(central, overlays, nil)
}

// LoadProject reads the central store, merges each overlay over it, and loads
// any listed eager files as local rules. Items are added in order and a later
// source wins on an identical ID, so overlays override the store and eager files
// override both.
func LoadProject(central string, overlays []string, eagerFiles []EagerFile) (*Store, error) {
	groups := make([][]Item, 0, len(overlays)+2)

	storeItems, err := loadItems(central, OriginStore)
	if err != nil {
		return nil, err
	}
	groups = append(groups, storeItems)

	for _, ov := range overlays {
		localItems, err := loadItems(ov, OriginLocal)
		if err != nil {
			return nil, err
		}
		groups = append(groups, localItems)
	}

	fileItems, err := loadEagerFileItems(eagerFiles)
	if err != nil {
		return nil, err
	}
	groups = append(groups, fileItems)

	return build(central, groups), nil
}

// LoadForKind loads the central store and resolves an optional kind filter. An
// empty kind selects every kind, so a user-supplied value can be passed straight
// through.
func LoadForKind(root, kind string) (*Store, []Kind, error) {
	st, err := Load(root)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(kind) == "" {
		return st, Kinds, nil
	}

	k, err := ParseKind(kind)
	if err != nil {
		return nil, nil, err
	}
	return st, []Kind{k}, nil
}

// DiscoverOverlays returns the recognised overlay directories that exist under
// projectDir, in OverlayDirNames order.
func DiscoverOverlays(projectDir string) []string {
	var out []string
	for _, name := range OverlayDirNames {
		p := filepath.Join(projectDir, name)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// Root returns the central store's directory.
func (s *Store) Root() string { return s.root }

// All returns every item in load order.
func (s *Store) All() []Item { return s.items }

// ByKind returns the items of a kind in load order.
func (s *Store) ByKind(kind Kind) []Item {
	var out []Item
	for _, it := range s.items {
		if it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}

// BySource returns the items of a kind from a single origin, in load order.
func (s *Store) BySource(kind Kind, origin Origin) []Item {
	var out []Item
	for _, it := range s.items {
		if it.Kind == kind && it.Source == origin {
			out = append(out, it)
		}
	}
	return out
}

// Count returns the number of items of a kind.
func (s *Store) Count(kind Kind) int {
	n := 0
	for _, it := range s.items {
		if it.Kind == kind {
			n++
		}
	}
	return n
}

// Search returns how each matching item of a kind matched the query, in load
// order. Non-matching items are omitted.
func (s *Store) Search(kind Kind, query string) []Match {
	var out []Match
	for _, it := range s.ByKind(kind) {
		if m, ok := it.findMatch(query); ok {
			out = append(out, m)
		}
	}
	return out
}

// Get returns the item with the given ID.
func (s *Store) Get(id string) (Item, bool) {
	i, ok := s.byID[id]
	if !ok {
		return Item{}, false
	}
	return s.items[i], true
}

// build assembles a Store from ordered item groups. Items are added in order;
// on an ID already seen, the later item replaces the earlier one in place, so
// overlay groups (passed after the store group) override the central store.
func build(root string, groups [][]Item) *Store {
	s := &Store{root: root, byID: make(map[string]int)}
	for _, group := range groups {
		for _, it := range group {
			if idx, ok := s.byID[it.ID]; ok {
				s.items[idx] = it
				continue
			}
			s.byID[it.ID] = len(s.items)
			s.items = append(s.items, it)
		}
	}
	return s
}

// loader walks one root and produces its items, stamped with an origin. IDs are
// relative to that root, so a local item and a central item are distinct unless
// their paths are identical.
type loader struct {
	root   string
	origin Origin
	items  []Item
}

// loadEagerFileItems reads each listed file as a local rule item. A listed file
// that no longer exists is skipped rather than failing the whole load — a
// project config can outlive a file it names.
func loadEagerFileItems(files []EagerFile) ([]Item, error) {
	var items []Item
	for _, f := range files {
		content, err := os.ReadFile(f.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		fm, body, err := parseItemFile(content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f.Path, err)
		}
		items = append(items, Item{
			Kind:        KindRules,
			Source:      OriginLocal,
			ID:          f.ID,
			Name:        pathBase(f.ID),
			Path:        f.Path,
			Description: fm.Description,
			Tags:        fm.Tags,
			Body:        body,
		})
	}
	return items, nil
}

func loadItems(root string, origin Origin) ([]Item, error) {
	l := &loader{root: root, origin: origin}
	for _, kind := range Kinds {
		if err := l.loadKind(kind); err != nil {
			return nil, err
		}
	}
	return l.items, nil
}

func (l *loader) loadKind(kind Kind) error {
	kindRoot := filepath.Join(l.root, string(kind))
	info, err := os.Stat(kindRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", kindRoot, err)
	}
	if !info.IsDir() {
		return nil
	}

	if kind == KindSkills {
		return l.loadSkills(kindRoot)
	}
	return l.loadFiles(kind, kindRoot)
}

// loadFiles loads the markdown files under a file-based kind (rules, knowledge).
func (l *loader) loadFiles(kind Kind, kindRoot string) error {
	return filepath.WalkDir(kindRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != MarkdownExt {
			return nil
		}

		it, err := l.buildItem(kind, kindRoot, path, path, false)
		if err != nil {
			return err
		}
		l.items = append(l.items, it)
		return nil
	})
}

// loadSkills loads directory-based skills. A skill is any directory containing
// a SKILL.md file; the walk does not descend into a skill's own directory.
func (l *loader) loadSkills(kindRoot string) error {
	return filepath.WalkDir(kindRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		skillFile := filepath.Join(path, SkillFile)
		if info, statErr := os.Stat(skillFile); statErr != nil || info.IsDir() {
			return nil // not a skill directory; keep descending
		}

		it, err := l.buildItem(KindSkills, kindRoot, path, skillFile, true)
		if err != nil {
			return err
		}
		l.items = append(l.items, it)
		return filepath.SkipDir // don't descend into the skill's internals
	})
}

// buildItem parses contentPath and assembles an Item. idPath is the path used to
// derive the ID and implicit tags (the file itself for file kinds, the skill
// directory for skills); isDir reports whether idPath is a directory.
func (l *loader) buildItem(kind Kind, kindRoot, idPath, contentPath string, isDir bool) (Item, error) {
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return Item{}, fmt.Errorf("read %s: %w", contentPath, err)
	}
	fm, body, err := parseItemFile(content)
	if err != nil {
		return Item{}, fmt.Errorf("parse %s: %w", contentPath, err)
	}

	id := idFor(l.root, idPath, isDir)
	return Item{
		Kind:        kind,
		Source:      l.origin,
		ID:          id,
		Name:        pathBase(id),
		Path:        contentPath,
		Description: fm.Description,
		Tags:        mergeTags(implicitTags(kindRoot, filepath.Dir(idPath)), fm.Tags),
		Body:        body,
	}, nil
}

// idFor returns the root-relative, slash-separated ID for a path, dropping the
// markdown extension for files.
func idFor(root, path string, isDir bool) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	if !isDir {
		rel = strings.TrimSuffix(rel, MarkdownExt)
	}
	return rel
}

// implicitTags returns the folder segments between the kind root and dir, each
// of which is an implicit tag on the item.
func implicitTags(kindRoot, dir string) []string {
	rel, err := filepath.Rel(kindRoot, dir)
	if err != nil || rel == "." || rel == "" {
		return nil
	}
	return strings.Split(filepath.ToSlash(rel), "/")
}

// mergeTags unions implicit and explicit tags, preserving order (implicit
// first) and dropping blanks and duplicates.
func mergeTags(implicit, explicit []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, group := range [][]string{implicit, explicit} {
		for _, t := range group {
			t = strings.TrimSpace(t)
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// pathBase returns the last slash-separated segment of an ID.
func pathBase(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

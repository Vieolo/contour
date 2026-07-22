package store

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Store is an in-memory index of every item in a contour store, loaded once
// from disk. Its exported read methods are safe for concurrent use.
type Store struct {
	root  string
	items []Item
	byID  map[string]int
}

// Load reads every rule, skill and knowledge item under root into memory.
// Items are ordered deterministically: by kind (rules, skills, knowledge), then
// lexicographically by path within each kind. A kind directory that does not
// exist simply contributes no items.
func Load(root string) (*Store, error) {
	s := &Store{root: root, byID: make(map[string]int)}
	for _, kind := range Kinds {
		if err := s.loadKind(kind); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Root returns the store's root directory.
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

// Get returns the item with the given ID.
func (s *Store) Get(id string) (Item, bool) {
	i, ok := s.byID[id]
	if !ok {
		return Item{}, false
	}
	return s.items[i], true
}

func (s *Store) loadKind(kind Kind) error {
	kindRoot := filepath.Join(s.root, string(kind))
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
		return s.loadSkills(kindRoot)
	}
	return s.loadFiles(kind, kindRoot)
}

// loadFiles loads the markdown files under a file-based kind (rules, knowledge).
func (s *Store) loadFiles(kind Kind, kindRoot string) error {
	return filepath.WalkDir(kindRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != MarkdownExt {
			return nil
		}

		it, err := s.buildItem(kind, kindRoot, path, path, false)
		if err != nil {
			return err
		}
		s.add(it)
		return nil
	})
}

// loadSkills loads directory-based skills. A skill is any directory containing
// a SKILL.md file; the walk does not descend into a skill's own directory.
func (s *Store) loadSkills(kindRoot string) error {
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

		it, err := s.buildItem(KindSkills, kindRoot, path, skillFile, true)
		if err != nil {
			return err
		}
		s.add(it)
		return filepath.SkipDir // don't descend into the skill's internals
	})
}

// buildItem parses contentPath and assembles an Item. idPath is the path used
// to derive the ID and implicit tags (the file itself for file kinds, the skill
// directory for skills); isDir reports whether idPath is a directory.
func (s *Store) buildItem(kind Kind, kindRoot, idPath, contentPath string, isDir bool) (Item, error) {
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return Item{}, fmt.Errorf("read %s: %w", contentPath, err)
	}
	fm, body, err := parseItemFile(content)
	if err != nil {
		return Item{}, fmt.Errorf("parse %s: %w", contentPath, err)
	}

	id := s.idFor(idPath, isDir)
	return Item{
		Kind:        kind,
		ID:          id,
		Name:        pathBase(id),
		Path:        contentPath,
		Description: fm.Description,
		Tags:        mergeTags(implicitTags(kindRoot, filepath.Dir(idPath)), fm.Tags),
		Body:        body,
	}, nil
}

func (s *Store) add(it Item) {
	s.byID[it.ID] = len(s.items)
	s.items = append(s.items, it)
}

// idFor returns the store-root-relative, slash-separated ID for a path,
// dropping the markdown extension for files.
func (s *Store) idFor(path string, isDir bool) string {
	rel, err := filepath.Rel(s.root, path)
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

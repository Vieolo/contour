// Package scaffold creates a contour store on disk — its directory structure and
// the sample files that document the layout — and moves one between locations.
//
// Both the `init` command and the first-use auto-creation that happens when no
// store exists go through here, so a store is identical however it came about.
package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vieolo/contour/internal/store"
)

// Create builds a new store at path — its directory structure and sample files —
// reporting whether it did so. Every kind directory is created even when no
// sample lands in it, so the layout is discoverable from an otherwise empty
// store.
//
// It only ever creates a store, never tops one up. A path that already holds
// anything is left exactly as it is and Create reports false. The samples exist
// to illustrate the layout on day one; once a store has content, they are the
// user's to edit or delete, and putting back a sample someone removed — or a
// kind directory they have no use for — would undo their choice every time the
// command ran. An existing store with pieces missing is not broken: the loader
// treats an absent kind directory as empty.
func Create(path string) (bool, error) {
	occupied, err := HasContent(path)
	if err != nil {
		return false, err
	}
	if occupied {
		return false, nil
	}

	dirs := []string{path, filepath.Join(path, store.BootstrapDir)}
	for _, k := range store.Kinds {
		dirs = append(dirs, filepath.Join(path, string(k)))
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	for _, f := range seedFiles() {
		target := filepath.Join(path, filepath.FromSlash(f.rel))
		if err := writeIfAbsent(target, f.body); err != nil {
			return false, err
		}
	}
	return true, nil
}

// gitKeep holds an otherwise empty kind directory in git, which tracks files,
// not directories. The loader reads only .md files, so it is never an item.
const gitKeep = ".gitkeep"

// CreateLocal gives the project at projectDir a local store — the overlay
// contour merges over the central store when it runs there — returning the
// overlay in effect and whether this call created it.
//
// A project that already has one is left as it is, and its overlay returned.
// "Already has one" means any recognised overlay folder holding a rules/,
// skills/ or knowledge/ directory. It cannot mean "the folder exists": .agents/
// and .claude/ are shared with other tools, so their presence says nothing
// about contour. For the same reason, scaffolding into an existing .agents/
// never overwrites a file another tool put there.
//
// Unlike the central store, no sample items are written. A central sample is
// inert until a profile selects it; a local item is live the moment it exists
// — local rules load eagerly into every session in the project, ahead of the
// central ones. A sample here would put placeholder text into real agent
// context. The formats are documented in the overlay's README instead, which
// the loader never reads: it looks only inside the kind directories.
func CreateLocal(projectDir string) (string, bool, error) {
	for _, name := range store.OverlayDirNames {
		overlay := filepath.Join(projectDir, name)
		for _, k := range store.Kinds {
			if info, err := os.Stat(filepath.Join(overlay, string(k))); err == nil && info.IsDir() {
				return overlay, false, nil
			}
		}
	}

	overlay := filepath.Join(projectDir, store.DefaultOverlayDir)
	for _, k := range store.Kinds {
		if err := writeIfAbsent(filepath.Join(overlay, string(k), gitKeep), ""); err != nil {
			return "", false, err
		}
	}
	if err := writeIfAbsent(filepath.Join(overlay, "README.md"), localReadmeSeed); err != nil {
		return "", false, err
	}
	return overlay, true, nil
}

// HasContent reports whether path exists and holds anything at all. A path that
// exists but is not a directory is an error: it can be neither a store nor the
// place to create one.
func HasContent(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s already exists and is not a directory", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	return len(entries) > 0, nil
}

// writeIfAbsent writes body to path only when path does not already exist,
// creating parent directories as needed. It is what lets CreateLocal scaffold
// into a .agents/ another tool already uses without clobbering its files, and a
// second line of defence for Create, which only seeds an empty directory.
func writeIfAbsent(path, body string) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return nil // already present
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

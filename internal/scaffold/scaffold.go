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
// creating parent directories as needed. Create only seeds an empty directory,
// so nothing should be there; this is a second line of defence, so that even a
// race cannot make the scaffolder overwrite a user's file.
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

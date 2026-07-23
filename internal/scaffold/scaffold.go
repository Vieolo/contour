// Package scaffold creates a contour store on disk: its directory structure and
// the sample files that document the layout.
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

// Create builds the store's directory structure and sample files at path. Every
// kind directory is created even when no sample lands in it, so the layout is
// discoverable from an otherwise empty store.
//
// It is safe to re-run: missing pieces are created and existing files are never
// overwritten.
func Create(path string) error {
	dirs := []string{path, filepath.Join(path, store.BootstrapDir)}
	for _, k := range store.Kinds {
		dirs = append(dirs, filepath.Join(path, string(k)))
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	for _, f := range seedFiles() {
		target := filepath.Join(path, filepath.FromSlash(f.rel))
		if err := writeIfAbsent(target, f.body); err != nil {
			return err
		}
	}
	return nil
}

// writeIfAbsent writes body to path only when path does not already exist,
// creating parent directories as needed. Existing files are never overwritten,
// so re-running never discards the user's own content.
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

// Package config resolves where the contour store lives. The store is the
// on-disk source of truth holding the rules, skills and knowledge that contour
// serves to AI agents.
//
// Resolution is side-effect free: it never creates or mutates anything on disk,
// so callers decide how to react to a missing store (for example the `init`
// command creates it, while read commands show onboarding guidance).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvVar names the environment variable that overrides the store location.
// When it is unset, DefaultPath is used.
const EnvVar = "CONTOUR_HOME"

// defaultDirName is the store directory created under the user's home directory
// when EnvVar is not set.
const defaultDirName = ".contour"

// Home is the resolved store location together with how it was determined.
type Home struct {
	// Path is the absolute path to the store directory.
	Path string

	// Explicit reports whether Path came from EnvVar rather than the built-in
	// default. It changes how a missing store is treated: an explicitly
	// configured path that does not exist is a mistake worth reporting as an
	// error, whereas a missing default simply means contour has not been set
	// up yet.
	Explicit bool

	// Exists reports whether Path is an existing directory.
	Exists bool
}

// DefaultPath returns the built-in store location (~/.contour), used when
// EnvVar is not set.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine user home directory: %w", err)
	}
	return filepath.Join(home, defaultDirName), nil
}

// Resolve determines where the contour store lives. It reads EnvVar, falling
// back to DefaultPath, normalises the result to an absolute path and records
// whether it currently exists. It performs no writes.
func Resolve() (Home, error) {
	var h Home

	if raw := strings.TrimSpace(os.Getenv(EnvVar)); raw != "" {
		path, err := normalize(raw)
		if err != nil {
			return Home{}, err
		}
		h.Path = path
		h.Explicit = true
	} else {
		path, err := DefaultPath()
		if err != nil {
			return Home{}, err
		}
		h.Path = path
	}

	h.Exists = isDir(h.Path)
	return h, nil
}

// normalize expands a leading ~ and makes the path absolute.
func normalize(path string) (string, error) {
	switch {
	case path == "~":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		path = home
	case strings.HasPrefix(path, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		path = filepath.Join(home, path[2:])
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}
	return abs, nil
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

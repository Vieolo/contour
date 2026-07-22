// Package config resolves where the contour store lives. The store is the
// on-disk source of truth holding the rules, skills and knowledge that contour
// serves to AI agents.
//
// Resolution is side-effect free: it never creates or mutates anything on disk,
// so callers decide how to react to a missing store (for example the `init`
// command creates it, while read commands show onboarding guidance).
//
// Production and development builds select different stores via build tags (see
// mode_prod.go / mode_dev.go) so that a `contour-dev` binary never touches the
// production store.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// profile describes one store "world": the environment variable that overrides
// its location and the directory, under the user's home, used when that
// variable is unset.
type profile struct {
	label          string
	envVar         string
	defaultDirName string
}

var (
	productionProfile  = profile{label: "production", envVar: "CONTOUR_HOME", defaultDirName: ".contour"}
	developmentProfile = profile{label: "development", envVar: "CONTOUR_HOME_DEV", defaultDirName: ".contour-dev"}
)

// EnvVar is the environment variable that overrides the active store's location
// (CONTOUR_HOME in production builds, CONTOUR_HOME_DEV in development builds).
var EnvVar = active.envVar

// Label names the active build's world: "production" or "development".
var Label = active.label

// Home is the resolved store location together with how it was determined.
type Home struct {
	// Path is the absolute path to the store directory.
	Path string

	// Explicit reports whether Path came from the store's environment variable
	// rather than the built-in default. It changes how a missing store is
	// treated: an explicitly configured path that does not exist is a mistake
	// worth reporting as an error, whereas a missing default simply means
	// contour has not been set up yet.
	Explicit bool

	// Exists reports whether Path is an existing directory.
	Exists bool
}

// Resolve determines where the active store lives (the development store in dev
// builds, the production store otherwise). It performs no writes.
func Resolve() (Home, error) {
	return active.resolve()
}

// ResolveProduction resolves the production store regardless of build tag. Dev
// builds use it as the source when seeding the development store.
func ResolveProduction() (Home, error) {
	return productionProfile.resolve()
}

func (p profile) resolve() (Home, error) {
	var h Home

	if raw := strings.TrimSpace(os.Getenv(p.envVar)); raw != "" {
		path, err := normalize(raw)
		if err != nil {
			return Home{}, err
		}
		h.Path = path
		h.Explicit = true
	} else {
		path, err := p.defaultPath()
		if err != nil {
			return Home{}, err
		}
		h.Path = path
	}

	h.Exists = isDir(h.Path)
	return h, nil
}

func (p profile) defaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine user home directory: %w", err)
	}
	return filepath.Join(home, p.defaultDirName), nil
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

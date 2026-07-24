// Package config resolves where the contour store lives. The store is the
// on-disk source of truth holding the rules, skills and knowledge that contour
// serves to AI agents.
//
// The location comes from a config file rather than the environment, because
// contour runs as an MCP server launched by an agent, which does not inherit the
// user's shell. A variable exported in a shell profile would be invisible to it.
// The config file lives outside the store — never inside it — so that moving the
// store cannot carry its own pointer away.
//
// Resolving is side-effect free: it never creates or mutates anything on disk,
// so callers decide how to react to a missing store (for example the `init`
// command creates it, while read commands scaffold it and carry on).
//
// Production and development builds select different stores and config files via
// build tags (see mode_prod.go / mode_dev.go) so that a `contour-dev` binary
// never touches production.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// profile describes one store "world": where its store lives by default, which
// config file records a relocation, and which environment variable overrides
// both.
type profile struct {
	label          string
	envVar         string
	defaultDirName string
	configFileName string
}

var (
	productionProfile = profile{
		label:          "production",
		envVar:         "CONTOUR_HOME",
		defaultDirName: "contour",
		configFileName: "config.yaml",
	}
	developmentProfile = profile{
		label:          "development",
		envVar:         "CONTOUR_HOME_DEV",
		defaultDirName: "contour-dev",
		configFileName: "config-dev.yaml",
	}
)

// EnvVar is the environment variable that overrides the active store's location
// (CONTOUR_HOME in production builds, CONTOUR_HOME_DEV in development builds).
//
// It takes precedence over the config file, but it is an escape hatch for CI and
// testing rather than the way users are expected to relocate a store — an agent
// launching contour as an MCP server will not have it set.
var EnvVar = active.envVar

// Label names the active build's world: "production" or "development".
var Label = active.label

// Source records how a store location was determined.
type Source string

const (
	SourceDefault Source = "default location"
	SourceConfig  Source = "config file"
	SourceEnv     Source = "environment variable"
)

// Home is the resolved store location together with how it was determined.
type Home struct {
	// Path is the absolute path to the store directory.
	Path string

	// Source says which of the three mechanisms decided Path.
	Source Source

	// Explicit reports whether the location was chosen deliberately — by the
	// config file or the environment — rather than falling back to the default.
	// It changes how a missing store is treated: a location the user chose and
	// that does not exist is a mistake worth reporting, whereas a missing
	// default simply means contour has not been set up yet.
	Explicit bool

	// Exists reports whether Path is an existing directory.
	Exists bool
}

// file is the on-disk config schema.
type file struct {
	// StorePath is where the store lives. Empty means the default location.
	StorePath string `yaml:"store_path"`
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

// ConfigPath returns the path of the active profile's config file, whether or
// not it exists yet.
func ConfigPath() (string, error) {
	return active.configPath()
}

// SetStorePath records path as the active store's location, creating the config
// file and its directory as needed. It returns the normalised store path and the
// config file it was written to.
//
// It does not create the store itself; callers decide whether to scaffold.
func SetStorePath(path string) (storePath string, configFile string, err error) {
	return active.setStorePath(path)
}

// EnvOverride returns the store path forced by the environment variable, if any.
// A caller that is about to change the config file can use it to warn that the
// change will have no effect.
func EnvOverride() string {
	return strings.TrimSpace(os.Getenv(active.envVar))
}

func (p profile) resolve() (Home, error) {
	// 1. The environment wins, for CI and testing.
	if raw := strings.TrimSpace(os.Getenv(p.envVar)); raw != "" {
		return newHome(raw, SourceEnv)
	}

	// 2. Then the config file, which is how users relocate a store.
	cfg, err := p.loadConfig()
	if err != nil {
		return Home{}, err
	}
	if strings.TrimSpace(cfg.StorePath) != "" {
		return newHome(cfg.StorePath, SourceConfig)
	}

	// 3. Otherwise the default location.
	path, err := p.defaultPath()
	if err != nil {
		return Home{}, err
	}
	return newHome(path, SourceDefault)
}

func newHome(rawPath string, source Source) (Home, error) {
	path, err := normalize(rawPath)
	if err != nil {
		return Home{}, err
	}
	return Home{
		Path:     path,
		Source:   source,
		Explicit: source != SourceDefault,
		Exists:   isDir(path),
	}, nil
}

func (p profile) defaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine user home directory: %w", err)
	}
	return filepath.Join(home, p.defaultDirName), nil
}

// configPath returns the profile's config file, in ~/.contour. It deliberately
// sits outside the store: a config kept inside would travel with the store when
// the user moved it, leaving contour unable to find either.
func (p profile) configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, p.configFileName), nil
}

// configDir is ~/.contour: a small hidden directory sitting beside the visible
// store at ~/contour. It is a fixed location and never moves, which is precisely
// what lets the store move freely.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine user home directory: %w", err)
	}
	return filepath.Join(home, ".contour"), nil
}

// loadConfig reads the profile's config file. A missing file is not an error: it
// simply means nothing has been configured yet.
func (p profile) loadConfig() (file, error) {
	path, err := p.configPath()
	if err != nil {
		return file{}, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return file{}, nil
	}
	if err != nil {
		return file{}, fmt.Errorf("read %s: %w", path, err)
	}

	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return file{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

func (p profile) setStorePath(path string) (string, string, error) {
	storePath, err := normalize(path)
	if err != nil {
		return "", "", err
	}
	configFile, err := p.configPath()
	if err != nil {
		return "", "", err
	}

	// Preserve any other settings already in the file.
	cfg, err := p.loadConfig()
	if err != nil {
		return "", "", err
	}
	cfg.StorePath = storePath

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", "", fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configFile), 0o755); err != nil {
		return "", "", fmt.Errorf("create %s: %w", filepath.Dir(configFile), err)
	}
	if err := os.WriteFile(configFile, data, 0o644); err != nil {
		return "", "", fmt.Errorf("write %s: %w", configFile, err)
	}
	return storePath, configFile, nil
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

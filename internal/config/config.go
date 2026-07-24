// Package config resolves where the contour store lives. The store is the
// on-disk source of truth holding the rules, skills and knowledge that contour
// serves to AI agents.
//
// A config file is the only way to relocate a store; contour reads no
// environment variables. Two reasons: contour usually runs as an MCP server
// launched by an agent, which does not inherit the user's shell, so a variable
// exported in a shell profile would be invisible to it; and offering a second
// mechanism only invites confusion about which one is in force — a poor trade
// when the readers are agents.
//
// The config file lives outside the store, never inside it, so that moving the
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

// profile describes one store "world": where its store lives by default, and
// which config file records a relocation.
type profile struct {
	label          string
	defaultDirName string
	configFileName string
}

var (
	productionProfile = profile{
		label:          "production",
		defaultDirName: "contour",
		configFileName: "config.yaml",
	}
	developmentProfile = profile{
		label:          "development",
		defaultDirName: "contour-dev",
		configFileName: "config-dev.yaml",
	}
)

// Label names the active build's world: "production" or "development".
var Label = active.label

// Source records how a store location was determined.
type Source string

const (
	SourceDefault Source = "default location"
	SourceConfig  Source = "config file"
)

// Home is the resolved store location together with how it was determined.
type Home struct {
	// Path is the absolute path to the store directory.
	Path string

	// Source says whether Path came from the config file or the default.
	Source Source

	// Explicit reports whether the location was chosen deliberately, through the
	// config file, rather than falling back to the default. It changes how a
	// missing store is treated: a location the user chose and that does not
	// exist is a mistake worth reporting, whereas a missing default simply means
	// contour has not been set up yet.
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

// NormalizePath expands a leading ~ and makes a path absolute, exactly as
// SetStorePath does when recording one. A caller that must compare or act on a
// target path before recording it uses this so the two always agree.
func NormalizePath(path string) (string, error) {
	return normalize(path)
}

// SetStorePath records path as the active store's location, creating the config
// file and its directory as needed. It returns the normalised store path and the
// config file it was written to.
//
// It does not create the store itself; callers decide whether to scaffold.
func SetStorePath(path string) (storePath string, configFile string, err error) {
	return active.setStorePath(path)
}

// EnsureFile writes a default config file if none exists yet, and reports its
// path and whether it had to be created.
//
// Having the file there from the outset — with its fields present and commented
// — means a user or an agent can open it and edit a known setting, instead of
// having to discover that a config file is possible and then guess its schema.
// A generated file leaves behaviour unchanged: an empty store_path means the
// default location.
func EnsureFile() (path string, created bool, err error) {
	return active.ensureFile()
}

func (p profile) resolve() (Home, error) {
	cfg, err := p.loadConfig()
	if err != nil {
		return Home{}, err
	}
	if strings.TrimSpace(cfg.StorePath) != "" {
		return newHome(cfg.StorePath, SourceConfig)
	}

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

	if err := p.writeConfig(cfg); err != nil {
		return "", "", err
	}
	return storePath, configFile, nil
}

// ensureFile writes a default config file when none exists yet.
func (p profile) ensureFile() (string, bool, error) {
	path, err := p.configPath()
	if err != nil {
		return "", false, err
	}

	switch _, statErr := os.Stat(path); {
	case statErr == nil:
		return path, false, nil
	case !errors.Is(statErr, os.ErrNotExist):
		return "", false, fmt.Errorf("stat %s: %w", path, statErr)
	}

	if err := p.writeConfig(file{}); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func (p profile) writeConfig(f file) error {
	path, err := p.configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, renderConfig(f), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// renderConfig produces the config file's contents, with every field present and
// documented.
//
// The file is rendered from this template rather than marshalled from the struct
// so that the comments survive every write: someone opening the file — a person
// or an agent — should find the available settings spelled out rather than have
// to work out the schema. The cost is that hand-written comments are replaced
// whenever contour rewrites the file.
func renderConfig(f file) []byte {
	var b strings.Builder

	b.WriteString("# contour configuration\n")
	b.WriteString("#\n")
	b.WriteString("# This file is contour's only configuration. contour reads no environment\n")
	b.WriteString("# variables, so every setting lives here.\n")
	b.WriteString("#\n")
	b.WriteString("# It sits outside the store on purpose: were it kept inside, moving the\n")
	b.WriteString("# store would carry its own pointer away with it.\n")
	b.WriteString("\n")
	b.WriteString("# store_path: the directory holding your rules, skills and knowledge.\n")
	b.WriteString("#   Leave it empty to use the default location (~/" + active.defaultDirName + ").\n")
	b.WriteString("#   Prefer `" + Program + " set-home <path>` over editing this by hand: it\n")
	b.WriteString("#   moves the store's content to the new location, or creates one there\n")
	b.WriteString("#   if you do not have a store yet. Editing this line only repoints\n")
	b.WriteString("#   contour, leaving your content where it is.\n")
	fmt.Fprintf(&b, "store_path: %q\n", f.StorePath)

	return []byte(b.String())
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

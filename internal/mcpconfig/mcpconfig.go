// Package mcpconfig edits the JSON file an MCP client reads to learn which
// servers to launch — .mcp.json in a project, for Claude Code.
//
// The file belongs to the user, not to contour: it may list other servers and
// carry settings contour knows nothing about. Everything except contour's own
// entry is therefore round-tripped untouched.
package mcpconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultFile is the per-project config an MCP client looks for.
const DefaultFile = ".mcp.json"

// serversKey is the object holding one entry per server.
const serversKey = "mcpServers"

// Entry describes how to launch one server.
type Entry struct {
	// Command is the executable. It should be an absolute path: a client may
	// start the server without a login shell, and so without the PATH that
	// would make a bare command name resolve.
	Command string `json:"command"`

	// Args are passed to the command.
	Args []string `json:"args,omitempty"`
}

// Upsert adds or replaces the named server in the config at path, creating the
// file when it does not exist. It reports whether an entry was replaced.
//
// Other servers and any unrecognised top-level fields are preserved verbatim.
// Key order is not: JSON objects are unordered and Go writes them sorted, so an
// existing file may come back with its keys rearranged.
func Upsert(path, name string, entry Entry) (replaced bool, err error) {
	doc, err := load(path)
	if err != nil {
		return false, err
	}

	servers := map[string]json.RawMessage{}
	if raw, ok := doc[serversKey]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return false, fmt.Errorf("parse %q in %s: %w", serversKey, path, err)
		}
	}

	_, replaced = servers[name]
	encodedEntry, err := json.Marshal(entry)
	if err != nil {
		return false, fmt.Errorf("encode the %q entry: %w", name, err)
	}
	servers[name] = encodedEntry

	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return false, fmt.Errorf("encode %q: %w", serversKey, err)
	}
	doc[serversKey] = encodedServers

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode %s: %w", path, err)
	}
	out = append(out, '\n')

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return replaced, nil
}

// load reads the config as raw top-level fields. A missing or empty file is an
// empty document rather than an error, so the first run needs no special case.
//
// Malformed JSON is an error and nothing is written: the file may hold another
// tool's configuration, and overwriting it to fix a parse failure would be a
// poor trade.
func load(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return map[string]json.RawMessage{}, nil
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", path, err)
	case len(bytes.TrimSpace(data)) == 0:
		return map[string]json.RawMessage{}, nil
	}

	doc := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON (%w); fix or remove it — contour will not overwrite it", path, err)
	}
	return doc, nil
}

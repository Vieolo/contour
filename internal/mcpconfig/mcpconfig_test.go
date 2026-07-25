package mcpconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func decode(t *testing.T, path string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(read(t, path)), &doc); err != nil {
		t.Fatalf("decode %s: %v\n%s", path, err, read(t, path))
	}
	return doc
}

func servers(t *testing.T, path string) map[string]any {
	t.Helper()
	doc := decode(t, path)
	s, ok := doc[serversKey].(map[string]any)
	if !ok {
		t.Fatalf("%s has no %q object: %v", path, serversKey, doc)
	}
	return s
}

func TestUpsertCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFile)

	replaced, err := Upsert(path, "contour", Entry{
		Command: "/opt/homebrew/bin/contour",
		Args:    []string{"mcp", "--bootstrap", "python"},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if replaced {
		t.Error("replaced = true when creating a new file")
	}

	entry, ok := servers(t, path)["contour"].(map[string]any)
	if !ok {
		t.Fatal("no contour entry written")
	}
	if got := entry["command"]; got != "/opt/homebrew/bin/contour" {
		t.Errorf("command = %v", got)
	}
	if got := entry["args"].([]any); len(got) != 3 || got[0] != "mcp" {
		t.Errorf("args = %v", got)
	}
}

func TestUpsertPreservesOtherServersAndFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFile)
	existing := `{
  "mcpServers": {
    "other": { "command": "othertool", "args": ["serve"], "env": { "K": "V" } }
  },
  "somethingElse": { "keep": true }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Upsert(path, "contour", Entry{Command: "/usr/local/bin/contour", Args: []string{"mcp"}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	s := servers(t, path)
	other, ok := s["other"].(map[string]any)
	if !ok {
		t.Fatal("the other server was dropped")
	}
	if other["command"] != "othertool" {
		t.Errorf("other.command = %v", other["command"])
	}
	// Fields contour knows nothing about must survive untouched.
	env, ok := other["env"].(map[string]any)
	if !ok || env["K"] != "V" {
		t.Errorf("other.env = %v, want {K:V}", other["env"])
	}
	if _, ok := s["contour"]; !ok {
		t.Error("contour entry missing")
	}

	doc := decode(t, path)
	extra, ok := doc["somethingElse"].(map[string]any)
	if !ok || extra["keep"] != true {
		t.Errorf("unrelated top-level field lost: %v", doc["somethingElse"])
	}
}

func TestUpsertReplacesExistingEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFile)

	if _, err := Upsert(path, "contour", Entry{Command: "/old/contour", Args: []string{"mcp"}}); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	replaced, err := Upsert(path, "contour", Entry{
		Command: "/new/contour", Args: []string{"mcp", "--bootstrap", "go"},
	})
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if !replaced {
		t.Error("replaced = false when overwriting an existing entry")
	}

	entry := servers(t, path)["contour"].(map[string]any)
	if entry["command"] != "/new/contour" {
		t.Errorf("command = %v, want /new/contour", entry["command"])
	}
}

func TestUpsertRefusesMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFile)
	broken := `{ "mcpServers": { oops`
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Upsert(path, "contour", Entry{Command: "/bin/contour"}); err == nil {
		t.Fatal("Upsert accepted malformed JSON")
	}
	// The user's file must be left exactly as it was.
	if got := read(t, path); got != broken {
		t.Errorf("file was modified despite the error:\n%s", got)
	}
}

func TestUpsertTreatsEmptyFileAsEmptyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFile)
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Upsert(path, "contour", Entry{Command: "/bin/contour", Args: []string{"mcp"}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, ok := servers(t, path)["contour"]; !ok {
		t.Error("contour entry missing")
	}
}

func TestUpsertWritesReadableJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFile)
	if _, err := Upsert(path, "contour", Entry{Command: "/bin/contour", Args: []string{"mcp"}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	out := read(t, path)
	// Indented, not a single dense line — someone will read and edit this.
	if !strings.Contains(out, "\n  ") {
		t.Errorf("output is not indented:\n%s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("output does not end with a newline")
	}
}

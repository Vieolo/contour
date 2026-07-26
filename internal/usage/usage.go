// Package usage records how an agent uses the store during an MCP session, so
// contour can later show which rules, skills and knowledge earn their place —
// and, most usefully, what agents searched for and could not find.
//
// It captures engagement, not effectiveness: a fetched item is one the agent
// reached for, not necessarily one that helped. The single most actionable
// signal is the gap — a search that returned nothing.
//
// Logs are plain JSONL, one file per session, grouped into a directory per
// project so per-project stats stay cheap. Each line is self-contained (it
// carries the project path and profile), so the folder layout is an
// optimisation, not something the reader depends on.
package usage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/vieolo/contour/internal/config"
)

// surfaceMCP marks events produced by the MCP server. The CLI is not logged in
// v1; the field exists so it can be told apart if the CLI is added later.
const surfaceMCP = "mcp"

// Event is one logged usage record. The Logger fills the ambient fields (time,
// surface, project, profile); callers supply only the event-specific ones.
type Event struct {
	Time    string `json:"ts"`
	Surface string `json:"surface"`
	Project string `json:"project"`
	Profile string `json:"profile,omitempty"`
	Type    string `json:"event"`
	ID      string `json:"id,omitempty"`
	Query   string `json:"query,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Results *int   `json:"results,omitempty"`
	Found   *bool  `json:"found,omitempty"`
}

// Logger appends the events of one session to its own JSONL file.
//
// Its methods are safe to call on a nil Logger — they no-op — so a caller that
// could not open a log (or has logging disabled) can call them unconditionally
// rather than guard every site.
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	project string
	profile string
}

// Open starts a session log for a session's bootstrap profiles. The project is
// the working directory the process was started in — the project the client
// spawned contour for.
//
// Several profiles are recorded as one comma-separated value rather than a JSON
// list, so a log written before profiles could compose still parses.
func Open(profiles []string) (*Logger, error) {
	profile := strings.Join(profiles, ",")

	project, err := projectDir()
	if err != nil {
		return nil, err
	}
	base, err := config.UsageDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(base, projectGroup(project))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	name, err := sessionFileName()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open session log: %w", err)
	}

	l := &Logger{file: f, project: project, profile: profile}
	l.record(Event{Type: "session_start"})
	return l, nil
}

// Close closes the session log.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// Get records a get by item ID and whether the item was found.
func (l *Logger) Get(id string, found bool) {
	if l == nil {
		return
	}
	l.record(Event{Type: "get", ID: id, Found: &found})
}

// Search records a search: the query, an optional kind filter, and how many
// items matched — zero being the gap signal.
func (l *Logger) Search(query, kind string, results int) {
	if l == nil {
		return
	}
	l.record(Event{Type: "search", Query: query, Kind: kind, Results: &results})
}

// List records a list, optionally filtered by kind.
func (l *Logger) List(kind string) {
	if l == nil {
		return
	}
	l.record(Event{Type: "list", Kind: kind})
}

func (l *Logger) record(e Event) {
	if l == nil || l.file == nil {
		return
	}
	e.Time = time.Now().UTC().Format(time.RFC3339)
	e.Surface = surfaceMCP
	e.Project = l.project
	e.Profile = l.profile

	line, err := json.Marshal(e)
	if err != nil {
		return // logging must never break a tool call
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.file.Write(append(line, '\n'))
}

// projectDir is the absolute working directory the process was started in.
func projectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine the working directory: %w", err)
	}
	return filepath.Abs(wd)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// projectGroup derives a stable, readable, filesystem-safe directory name for a
// project path: its base name so a person recognises it, plus a short hash of
// the full path so two projects sharing a base name never collide.
func projectGroup(dir string) string {
	sum := sha256.Sum256([]byte(dir))
	short := hex.EncodeToString(sum[:])[:8]

	base := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(filepath.Base(dir)), "-"), "-")
	if base == "" {
		base = "root"
	}
	return base + "-" + short
}

// sessionFileName is a timestamped, collision-resistant name so a directory
// listing sorts sessions chronologically.
func sessionFileName() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:]) + ".jsonl", nil
}

package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vieolo/contour/internal/config"
)

// writeSession writes a session log into the active usage directory, asking
// config where that is rather than assuming "usage" — under a dev build it is
// "usage-dev".
func writeSession(t *testing.T, group, name string, lines ...string) {
	t.Helper()
	base, err := config.UsageDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, group)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAggregate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeSession(t, "a-1111", "s1.jsonl",
		`{"ts":"2026-07-01T10:00:00Z","event":"session_start","project":"/proj/a"}`,
		`{"ts":"2026-07-01T10:01:00Z","event":"get","project":"/proj/a","id":"rules/x","found":true}`,
		`{"ts":"2026-07-01T10:02:00Z","event":"get","project":"/proj/a","id":"rules/x","found":true}`,
		`{"ts":"2026-07-01T10:03:00Z","event":"search","project":"/proj/a","query":"docker","results":0}`,
		`{"ts":"2026-07-01T10:04:00Z","event":"search","project":"/proj/a","query":"k8s","results":0}`,
		`{"ts":"2026-07-01T10:05:00Z","event":"get","project":"/proj/a","id":"gone","found":false}`,
	)
	writeSession(t, "b-2222", "s2.jsonl",
		`{"ts":"2026-07-02T09:00:00Z","event":"session_start","project":"/proj/b"}`,
		`{"ts":"2026-07-02T09:01:00Z","event":"search","project":"/proj/b","query":"docker","results":0}`,
		`{"ts":"2026-07-02T09:02:00Z","event":"get","project":"/proj/b","id":"skills/y","found":true}`,
	)

	r, err := Aggregate(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sessions != 2 {
		t.Errorf("sessions = %d, want 2", r.Sessions)
	}
	if r.Projects != 2 {
		t.Errorf("projects = %d, want 2", r.Projects)
	}
	// docker appears in both projects → count 2, ranked first.
	if len(r.Gaps) != 2 || r.Gaps[0].Key != "docker" || r.Gaps[0].Count != 2 {
		t.Errorf("gaps = %+v, want docker(2) first", r.Gaps)
	}
	if len(r.Fetches) != 2 || r.Fetches[0].Key != "rules/x" || r.Fetches[0].Count != 2 {
		t.Errorf("fetches = %+v, want rules/x(2) first", r.Fetches)
	}
	if len(r.MissingFetches) != 1 || r.MissingFetches[0].Key != "gone" {
		t.Errorf("missing = %+v, want gone(1)", r.MissingFetches)
	}
	if keys := r.FetchedKeys(); !keys["rules/x"] || keys["gone"] {
		t.Errorf("FetchedKeys = %v", keys)
	}
}

func TestAggregateProjectFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeSession(t, "a", "s1.jsonl",
		`{"ts":"2026-07-01T10:00:00Z","event":"session_start","project":"/work/alpha"}`,
		`{"ts":"2026-07-01T10:01:00Z","event":"search","project":"/work/alpha","query":"docker","results":0}`,
	)
	writeSession(t, "b", "s2.jsonl",
		`{"ts":"2026-07-02T10:00:00Z","event":"session_start","project":"/work/beta"}`,
		`{"ts":"2026-07-02T10:01:00Z","event":"search","project":"/work/beta","query":"k8s","results":0}`,
	)

	r, err := Aggregate(Options{ProjectSubstr: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sessions != 1 || r.Projects != 1 {
		t.Errorf("sessions=%d projects=%d, want 1/1", r.Sessions, r.Projects)
	}
	if len(r.Gaps) != 1 || r.Gaps[0].Key != "docker" {
		t.Errorf("gaps = %+v, want only docker", r.Gaps)
	}
}

func TestAggregateSinceFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeSession(t, "a", "s1.jsonl",
		`{"ts":"2020-01-01T00:00:00Z","event":"search","project":"/p","query":"old","results":0}`,
		`{"ts":"2030-01-01T00:00:00Z","event":"search","project":"/p","query":"new","results":0}`,
	)

	cutoff, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")
	r, err := Aggregate(Options{Since: cutoff})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Gaps) != 1 || r.Gaps[0].Key != "new" {
		t.Errorf("gaps = %+v, want only new", r.Gaps)
	}
}

func TestAggregateNoDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // nothing has ever been logged

	r, err := Aggregate(Options{})
	if err != nil {
		t.Fatalf("Aggregate on a missing usage dir: %v", err)
	}
	if r.Sessions != 0 || len(r.Gaps) != 0 {
		t.Errorf("expected an empty report, got %+v", r)
	}
}

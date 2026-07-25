package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectGroupStableReadableUnique(t *testing.T) {
	a := projectGroup("/Users/x/projects/myapp")

	if a != projectGroup("/Users/x/projects/myapp") {
		t.Error("projectGroup is not stable for the same path")
	}
	// Same base name, different path, must not collide.
	if b := projectGroup("/Users/y/work/myapp"); a == b {
		t.Errorf("collision: %q == %q for different paths", a, b)
	}
	if !strings.HasPrefix(a, "myapp-") {
		t.Errorf("group %q lacks a readable base name", a)
	}
	if strings.ContainsAny(a, `/\ `) {
		t.Errorf("group %q is not filesystem-safe", a)
	}
}

func TestProjectGroupSlugsAwkwardNames(t *testing.T) {
	g := projectGroup("/tmp/My Cool App!")
	if !strings.HasPrefix(g, "my-cool-app-") {
		t.Errorf("group = %q, want a slugged base 'my-cool-app'", g)
	}
}

func TestLoggerWritesSelfContainedJSONL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := t.TempDir()
	t.Chdir(project)

	wantProject, err := projectDir()
	if err != nil {
		t.Fatal(err)
	}

	l, err := Open("python")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.Get("skills/python/release", true)
	l.Get("does/not/exist", false)
	l.Search("docker", "", 0) // the gap signal
	l.List("skills")
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// One session file, under a per-project directory in ~/.contour/usage.
	files, err := filepath.Glob(filepath.Join(home, ".contour", "usage", "*", "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("want exactly one session file, got %v (err %v)", files, err)
	}
	if base := filepath.Base(filepath.Dir(files[0])); !strings.HasPrefix(base, filepath.Base(project)) &&
		!strings.Contains(base, "-") {
		t.Errorf("session dir %q is not a project group", base)
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var events []Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("line is not valid JSON: %v\n%s", err, line)
		}
		// Every line is self-contained: ambient fields always present.
		if e.Time == "" || e.Surface != "mcp" || e.Project != wantProject || e.Profile != "python" {
			t.Errorf("missing ambient fields: %+v", e)
		}
		events = append(events, e)
	}

	// session_start, 2 gets, 1 search, 1 list.
	if len(events) != 5 {
		t.Fatalf("got %d events, want 5: %+v", len(events), events)
	}
	if events[0].Type != "session_start" {
		t.Errorf("first event = %q, want session_start", events[0].Type)
	}

	byType := map[string][]Event{}
	for _, e := range events {
		byType[e.Type] = append(byType[e.Type], e)
	}
	if got := byType["get"]; len(got) != 2 || *got[0].Found != true || *got[1].Found != false {
		t.Errorf("get events wrong: %+v", got)
	}
	if s := byType["search"]; len(s) != 1 || s[0].Query != "docker" || *s[0].Results != 0 {
		t.Errorf("search event wrong: %+v", s)
	}
	if ls := byType["list"]; len(ls) != 1 || ls[0].Kind != "skills" {
		t.Errorf("list event wrong: %+v", ls)
	}
}

func TestNilLoggerIsSafe(t *testing.T) {
	var l *Logger // logging disabled / failed to open
	l.Get("x", true)
	l.Search("y", "", 3)
	l.List("")
	if err := l.Close(); err != nil {
		t.Errorf("Close on nil = %v, want nil", err)
	}
}

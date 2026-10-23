package cmd

import (
	"testing"

	"github.com/vieolo/contour/internal/project"
)

// A session's reach starts from its opening preset and only ever widens:
// loading a preset adds to a session, never takes anything away. Outside a
// session — the terminal — nothing is scoped.
func TestSessionScope(t *testing.T) {
	t.Cleanup(func() {
		mcpSession.Lock()
		mcpSession.active, mcpSession.central = false, false
		mcpSession.Unlock()
	})

	if !reachesCentral() {
		t.Fatal("the terminal is scoped away from the central store")
	}

	startSession(project.Preset{Name: "default", Central: false})
	if reachesCentral() {
		t.Error("a central: false session reaches the central store")
	}

	widenSession(project.Preset{Name: "other-local", Central: false})
	if reachesCentral() {
		t.Error("loading another central: false preset opened the central store")
	}

	widenSession(project.Preset{Name: "full", Central: true})
	if !reachesCentral() {
		t.Error("loading a preset that uses the central store did not open it")
	}

	widenSession(project.Preset{Name: "back-to-local", Central: false})
	if !reachesCentral() {
		t.Error("loading a central: false preset narrowed the session again")
	}
}

package mcpserver

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/vieolo/contour/internal/bootstrap"
)

func TestToolErrorRendersJSON(t *testing.T) {
	e := NotFound(`no item with ID "x"`, "call the list tool")

	var got map[string]any
	if err := json.Unmarshal([]byte(e.Error()), &got); err != nil {
		t.Fatalf("Error() is not valid JSON: %v\n%s", err, e.Error())
	}
	if got["status"] != StatusNotFound {
		t.Errorf("status = %v, want %q", got["status"], StatusNotFound)
	}
	if got["message"] != `no item with ID "x"` {
		t.Errorf("message = %v", got["message"])
	}
	if got["next_action"] != "call the list tool" {
		t.Errorf("next_action = %v", got["next_action"])
	}
}

func TestToolErrorOmitsEmptyNextAction(t *testing.T) {
	var got map[string]any
	if err := json.Unmarshal([]byte(NewToolError(StatusError, "boom", "").Error()), &got); err != nil {
		t.Fatal(err)
	}
	if _, present := got["next_action"]; present {
		t.Errorf("next_action should be omitted when empty, got %v", got)
	}
}

func TestToolErrorAsTarget(t *testing.T) {
	// asToolError relies on errors.As recognising an already-structured error.
	var err error = NotFound("x", "y")
	var te *ToolError
	if !errors.As(err, &te) {
		t.Fatal("errors.As failed to extract *ToolError")
	}
	if te.Status != StatusNotFound {
		t.Errorf("status = %q, want %q", te.Status, StatusNotFound)
	}
}

// The notice and its closer are written into space reserved for them. If they
// outgrow that reserve the instructions overrun the budget, which is the exact
// failure the budget exists to prevent — so the reserve is checked against the
// real strings at their widest.
func TestNoticeReserveCoversTheNotice(t *testing.T) {
	// Six-figure counts are far past anything realistic, so this is the widest
	// the numbers can plausibly render.
	ex := bootstrap.Excerpt{
		ShownRules: 999999, TotalRules: 999999,
		ShownChars: 999999, TotalChars: 999999,
		MenusIncluded: false,
	}
	if n := len(excerptNotice(ex)) + len(excerptCloser(ex)); n > noticeReserve {
		t.Errorf("notice + closer is %d chars, over the %d reserved", n, noticeReserve)
	}

	// And the singular wording, which must also fit.
	one := bootstrap.Excerpt{ShownRules: 1, TotalRules: 2, ShownChars: 10, TotalChars: 20}
	if n := len(excerptNotice(one)) + len(excerptCloser(one)); n > noticeReserve {
		t.Errorf("singular notice + closer is %d chars, over the %d reserved", n, noticeReserve)
	}
}

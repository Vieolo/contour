package mcpserver

import (
	"encoding/json"
	"errors"
	"testing"
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

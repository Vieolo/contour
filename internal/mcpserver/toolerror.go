package mcpserver

import "encoding/json"

// ToolError is a structured tool failure. Its Error method renders a compact
// JSON envelope, and the MCP SDK places that text in the error result (with
// IsError set), so an agent receives a predictable shape it can branch on —
// a status and a suggested next action — rather than a prose sentence it has to
// interpret.
//
// The envelope travels as the error's text, not as structured content: when a
// tool handler returns an error the SDK discards any result it also returned, so
// structured content is not available on the error path. For a small, always-
// text error payload that is an acceptable trade, and IsError already carries
// the machine "this failed" signal.
type ToolError struct {
	// Status is a short, stable machine code — one of the Status* constants —
	// so an agent can switch on it.
	Status string `json:"status"`

	// Message is the human- and model-readable description.
	Message string `json:"message"`

	// NextAction, when set, tells the agent what to do about it in terms of the
	// tools available to it (never a shell command it cannot run).
	NextAction string `json:"next_action,omitempty"`
}

// Status values contour uses. Kept to a small, stable set.
const (
	StatusNotFound        = "not_found"
	StatusInvalidArgument = "invalid_argument"
	StatusError           = "error"
)

func (e *ToolError) Error() string {
	b, err := json.Marshal(e)
	if err != nil {
		return e.Message
	}
	return string(b)
}

// NewToolError builds a structured tool error.
func NewToolError(status, message, nextAction string) *ToolError {
	return &ToolError{Status: status, Message: message, NextAction: nextAction}
}

// NotFound reports that a requested item does not exist.
func NotFound(message, nextAction string) *ToolError {
	return NewToolError(StatusNotFound, message, nextAction)
}

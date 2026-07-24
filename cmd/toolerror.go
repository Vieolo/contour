package cmd

import (
	"errors"

	"github.com/vieolo/contour/internal/mcpserver"
	"github.com/vieolo/contour/internal/store"
)

// asToolError converts a plain error from a shared helper into the structured
// envelope the MCP tools return, so every tool failure reaches an agent as the
// same {status, message, next_action} shape rather than an ad-hoc sentence.
//
// It is MCP-only: the CLI surfaces of these commands return their errors
// unwrapped, because a person reading the terminal wants the plain message, not
// JSON.
func asToolError(err error) error {
	if err == nil {
		return nil
	}

	// Already structured (e.g. a not-found built at the call site): leave it be.
	var te *mcpserver.ToolError
	if errors.As(err, &te) {
		return err
	}

	// A bad kind is the one agent-correctable case among the shared helpers.
	if errors.Is(err, store.ErrUnknownKind) {
		return mcpserver.NewToolError(mcpserver.StatusInvalidArgument, err.Error(),
			"set kind to one of rules, skills or knowledge, or omit it")
	}

	// Everything else (a missing or unreadable store) is not something the agent
	// can fix, so there is no next action to suggest — but it is still delivered
	// in the same envelope for a uniform shape.
	return mcpserver.NewToolError(mcpserver.StatusError, err.Error(), "")
}

package cmd

import "github.com/vieolo/contour/internal/usage"

// mcpUsage is the usage logger for the running MCP server. It is nil outside
// `contour mcp` and whenever usage logging is disabled; its methods are
// nil-safe, so the tool handlers record events unconditionally rather than
// guard every call site.
//
// Only the MCP surface records: those events are the agent signal the stats are
// about. The CLI, driven by a person, is intentionally not logged in v1.
var mcpUsage *usage.Logger

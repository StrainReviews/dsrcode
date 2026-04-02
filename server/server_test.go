package server_test

import (
	"testing"
)

// TestHookEndpoints verifies that POST /hooks/pre-tool-use with a valid JSON body
// containing tool_name, session_id, and cwd returns HTTP 200.
func TestHookEndpoints(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestActivityMapping verifies that mapHookToActivity correctly maps tool names
// to activity icons:
//   - "Edit" -> SmallImageKey "coding"
//   - "Bash" -> "terminal"
//   - "Grep" -> "searching"
//   - "Read" -> "reading"
//   - "Task" -> "thinking"
func TestActivityMapping(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestStatuslineEndpoint verifies that POST /statusline with valid statusline
// JSON returns HTTP 200 and updates session data.
func TestStatuslineEndpoint(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestConfigEndpoint verifies that POST /config with a preset name (e.g.
// {"preset":"hacker"}) returns HTTP 200 and triggers a config reload.
func TestConfigEndpoint(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestHealthEndpoint verifies that GET /health returns HTTP 200 with a JSON
// body containing connected status, session count, and uptime.
func TestHealthEndpoint(t *testing.T) {
	t.Skip("not implemented yet")
}

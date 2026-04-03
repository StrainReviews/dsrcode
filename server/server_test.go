package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tsanva/cc-discord-presence/server"
	"github.com/tsanva/cc-discord-presence/session"
)

// newTestServer creates a Server with a no-op onChange registry and optional onConfig.
func newTestServer(onConfig func(server.ConfigUpdatePayload)) (*server.Server, *session.SessionRegistry) {
	registry := session.NewRegistry(func() {})
	srv := server.NewServer(registry, onConfig)
	return srv, registry
}

// TestHookEndpoints verifies that POST /hooks/pre-tool-use with a valid JSON body
// containing tool_name, session_id, and cwd returns HTTP 200 and creates a session.
func TestHookEndpoints(t *testing.T) {
	srv, registry := newTestServer(nil)
	handler := srv.Handler()

	body := `{"tool_name":"Edit","session_id":"test-1","cwd":"/tmp/myproject"}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify session was created
	if registry.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", registry.SessionCount())
	}

	// Verify the session has correct SmallImageKey
	s := registry.GetSession("test-1")
	if s == nil {
		t.Fatal("session test-1 not found in registry")
	}
	if s.SmallImageKey != "coding" {
		t.Errorf("expected SmallImageKey 'coding', got %q", s.SmallImageKey)
	}

	// Test stop hook type
	body2 := `{"session_id":"test-1","cwd":"/tmp/myproject"}`
	req2 := httptest.NewRequest(http.MethodPost, "/hooks/stop", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("stop hook: expected status 200, got %d", w2.Code)
	}

	s2 := registry.GetSession("test-1")
	if s2 == nil {
		t.Fatal("session test-1 not found after stop hook")
	}
	if s2.SmallImageKey != "idle" {
		t.Errorf("after stop hook: expected SmallImageKey 'idle', got %q", s2.SmallImageKey)
	}
}

// TestActivityMapping verifies that MapHookToActivity correctly maps tool names
// to activity icons for all hook types and tools.
func TestActivityMapping(t *testing.T) {
	tests := []struct {
		hookType string
		toolName string
		wantIcon string
		wantText string
	}{
		{"pre-tool-use", "Edit", "coding", "Editing a file"},
		{"pre-tool-use", "Write", "coding", "Writing a file"},
		{"pre-tool-use", "Bash", "terminal", "Running a command"},
		{"pre-tool-use", "Grep", "searching", "Searching codebase"},
		{"pre-tool-use", "Glob", "searching", "Searching files"},
		{"pre-tool-use", "Read", "reading", "Reading a file"},
		{"pre-tool-use", "Task", "thinking", "Running a subtask"},
		{"pre-tool-use", "WebSearch", "searching", "Searching the web"},
		{"pre-tool-use", "WebFetch", "searching", "Fetching web content"},
		{"pre-tool-use", "UnknownTool", "thinking", "Processing..."},
		{"user-prompt-submit", "", "thinking", "Thinking..."},
		{"stop", "", "idle", "Finished"},
		{"notification", "", "idle", "Waiting for input"},
	}

	for _, tt := range tests {
		t.Run(tt.hookType+"/"+tt.toolName, func(t *testing.T) {
			icon, text := server.MapHookToActivity(tt.hookType, tt.toolName)
			if icon != tt.wantIcon {
				t.Errorf("icon: got %q, want %q", icon, tt.wantIcon)
			}
			if text != tt.wantText {
				t.Errorf("text: got %q, want %q", text, tt.wantText)
			}
		})
	}
}

// TestStatuslineEndpoint verifies that POST /statusline with valid statusline
// JSON returns HTTP 200 and updates session data.
func TestStatuslineEndpoint(t *testing.T) {
	srv, registry := newTestServer(nil)
	handler := srv.Handler()

	// First create a session via POST /hooks/pre-tool-use
	hookBody := `{"tool_name":"Edit","session_id":"test-1","cwd":"/tmp/myproject"}`
	hookReq := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(hookBody))
	hookReq.Header.Set("Content-Type", "application/json")

	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, hookReq)

	if w1.Code != http.StatusOK {
		t.Fatalf("hook setup: expected status 200, got %d", w1.Code)
	}

	// Then POST /statusline with session metadata
	statusBody := `{"session_id":"test-1","cwd":"/tmp/myproject","model":{"id":"opus-4.6","name":"Opus 4.6"},"total_tokens":150000,"total_cost_usd":0.12}`
	statusReq := httptest.NewRequest(http.MethodPost, "/statusline", strings.NewReader(statusBody))
	statusReq.Header.Set("Content-Type", "application/json")

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, statusReq)

	if w2.Code != http.StatusOK {
		t.Fatalf("statusline: expected status 200, got %d", w2.Code)
	}

	// Verify session was updated with model/tokens/cost
	s := registry.GetSession("test-1")
	if s == nil {
		t.Fatal("session test-1 not found after statusline update")
	}
	if s.Model != "Opus 4.6" {
		t.Errorf("expected Model 'Opus 4.6', got %q", s.Model)
	}
	if s.TotalTokens != 150000 {
		t.Errorf("expected TotalTokens 150000, got %d", s.TotalTokens)
	}
	if s.TotalCostUSD != 0.12 {
		t.Errorf("expected TotalCostUSD 0.12, got %f", s.TotalCostUSD)
	}
}

// TestConfigEndpoint verifies that POST /config with a preset name
// returns HTTP 200 with {"ok":true} and triggers the onConfig callback.
func TestConfigEndpoint(t *testing.T) {
	var receivedPreset string
	onConfig := func(payload server.ConfigUpdatePayload) {
		receivedPreset = payload.Preset
	}

	srv, _ := newTestServer(onConfig)
	handler := srv.Handler()

	body := `{"preset":"hacker"}`
	req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify response body contains {"ok":true}
	var resp map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp["ok"] {
		t.Error("expected ok=true in response")
	}

	// Verify onConfig was called with correct preset
	if receivedPreset != "hacker" {
		t.Errorf("expected onConfig called with preset 'hacker', got %q", receivedPreset)
	}
}

// TestExtendedHookPayload verifies that POST /hooks/pre-tool-use with a full payload
// including tool_input, hook_event_name, transcript_path, permission_mode returns 200 OK.
func TestExtendedHookPayload(t *testing.T) {
	srv, registry := newTestServer(nil)
	handler := srv.Handler()

	body := `{
		"tool_name": "Edit",
		"session_id": "ext-1",
		"cwd": "/tmp/myproject",
		"tool_input": {"file_path": "/tmp/myproject/src/main.go"},
		"hook_event_name": "PreToolUse",
		"transcript_path": "/home/user/.claude/transcript.json",
		"permission_mode": "auto-accept"
	}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session was created with extended fields
	s := registry.GetSession("ext-1")
	if s == nil {
		t.Fatal("session ext-1 not found in registry")
	}
	if s.LastFile != "main.go" {
		t.Errorf("LastFile = %q, want %q", s.LastFile, "main.go")
	}
	if s.LastFilePath != "/tmp/myproject/src/main.go" {
		t.Errorf("LastFilePath = %q, want %q", s.LastFilePath, "/tmp/myproject/src/main.go")
	}
}

// TestToolInputParsing is a table-driven test for extractToolContext with various tool types.
func TestToolInputParsing(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		wantFile string
		wantPath string
		wantCmd  string
		wantQry  string
	}{
		{
			name:     "Edit extracts file",
			toolName: "Edit",
			input:    `{"file_path":"/src/main.go"}`,
			wantFile: "main.go",
			wantPath: "/src/main.go",
		},
		{
			name:     "Write extracts file",
			toolName: "Write",
			input:    `{"file_path":"/lib/utils.ts"}`,
			wantFile: "utils.ts",
			wantPath: "/lib/utils.ts",
		},
		{
			name:     "Read extracts file",
			toolName: "Read",
			input:    `{"file_path":"C:/Users/dev/project/config.go"}`,
			wantFile: "config.go",
			wantPath: "C:/Users/dev/project/config.go",
		},
		{
			name:     "Bash extracts command",
			toolName: "Bash",
			input:    `{"command":"go test ./..."}`,
			wantCmd:  "go test ./...",
		},
		{
			name:     "Grep extracts pattern",
			toolName: "Grep",
			input:    `{"pattern":"TODO","path":"/src"}`,
			wantQry:  "TODO",
		},
		{
			name:     "Glob extracts pattern",
			toolName: "Glob",
			input:    `{"pattern":"**/*.go"}`,
			wantQry:  "**/*.go",
		},
		{
			name:     "Task returns empty (internal only)",
			toolName: "Task",
			input:    `{"prompt":"analyze this"}`,
		},
		{
			name:     "Agent returns empty (internal only)",
			toolName: "Agent",
			input:    `{"prompt":"investigate"}`,
		},
		{
			name:     "Unknown tool returns empty",
			toolName: "UnknownTool",
			input:    `{"data":"something"}`,
		},
		{
			name:     "Empty input returns empty",
			toolName: "Edit",
			input:    ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawInput json.RawMessage
			if tt.input != "" {
				rawInput = json.RawMessage(tt.input)
			}

			file, filePath, command, query := server.ExtractToolContext(tt.toolName, rawInput)
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if filePath != tt.wantPath {
				t.Errorf("filePath = %q, want %q", filePath, tt.wantPath)
			}
			if command != tt.wantCmd {
				t.Errorf("command = %q, want %q", command, tt.wantCmd)
			}
			if query != tt.wantQry {
				t.Errorf("query = %q, want %q", query, tt.wantQry)
			}
		})
	}
}

// TestSessionIdFallback verifies that POST /hooks/pre-tool-use with empty session_id
// returns 200 (not 400) and creates a session with synthetic ID "http-{projectName}".
func TestSessionIdFallback(t *testing.T) {
	srv, registry := newTestServer(nil)
	handler := srv.Handler()

	body := `{"tool_name":"Edit","session_id":"","cwd":"/home/user/myproject"}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session was created with synthetic ID
	s := registry.GetSession("http-myproject")
	if s == nil {
		t.Fatal("session 'http-myproject' not found in registry (synthetic ID should be used)")
	}
	if s.ProjectName != "myproject" {
		t.Errorf("ProjectName = %q, want %q", s.ProjectName, "myproject")
	}
}

// TestSessionIdFallbackNoCwd verifies that empty session_id with empty cwd
// creates a synthetic ID "http-unknown".
func TestSessionIdFallbackNoCwd(t *testing.T) {
	srv, registry := newTestServer(nil)
	handler := srv.Handler()

	body := `{"tool_name":"Edit","session_id":"","cwd":""}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	s := registry.GetSession("http-unknown")
	if s == nil {
		t.Fatal("session 'http-unknown' not found (empty cwd should use 'unknown')")
	}
}

// TestSessionIdPresent verifies that a hook with a valid session_id
// still works as before (no regression).
func TestSessionIdPresent(t *testing.T) {
	srv, registry := newTestServer(nil)
	handler := srv.Handler()

	body := `{"tool_name":"Edit","session_id":"valid-123","cwd":"/home/user/myproject"}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	s := registry.GetSession("valid-123")
	if s == nil {
		t.Fatal("session 'valid-123' not found")
	}
	if s.SmallImageKey != "coding" {
		t.Errorf("SmallImageKey = %q, want %q", s.SmallImageKey, "coding")
	}
}

// TestHealthEndpoint verifies that GET /health returns HTTP 200 with a JSON
// body containing status, session count, and uptime.
func TestHealthEndpoint(t *testing.T) {
	srv, _ := newTestServer(nil)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}

	// sessions should be 0 (no sessions registered yet)
	sessions, ok := resp["sessions"].(float64)
	if !ok {
		t.Fatalf("expected sessions to be a number, got %T", resp["sessions"])
	}
	if sessions != 0 {
		t.Errorf("expected 0 sessions, got %v", sessions)
	}

	// uptime should be present and non-empty
	uptime, ok := resp["uptime"].(string)
	if !ok || uptime == "" {
		t.Error("expected non-empty uptime string")
	}
}

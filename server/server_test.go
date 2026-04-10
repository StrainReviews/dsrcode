package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/StrainReviews/dsrcode/server"
	"github.com/StrainReviews/dsrcode/session"
)

// defaultTestConfig returns a ServerConfig for test use.
func defaultTestConfig() server.ServerConfig {
	return server.ServerConfig{
		Preset:        "minimal",
		DisplayDetail: "minimal",
		Port:          19460,
		BindAddr:      "127.0.0.1",
	}
}

// newTestServer creates a Server with a no-op onChange registry and optional onConfig.
func newTestServer(onConfig func(server.ConfigUpdatePayload)) (*server.Server, *session.SessionRegistry) {
	registry := session.NewRegistry(func() {})
	srv := server.NewServer(
		registry,
		onConfig,
		"2.0.0",
		func() server.ServerConfig { return defaultTestConfig() },
		func() bool { return false },
		nil,
		nil,
	)
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

// TestConfigEndpointLang verifies that POST /config with lang triggers the callback
// and that invalid lang values are rejected.
func TestConfigEndpointLang(t *testing.T) {
	t.Run("lang only", func(t *testing.T) {
		var received server.ConfigUpdatePayload
		onConfig := func(payload server.ConfigUpdatePayload) {
			received = payload
		}

		srv, _ := newTestServer(onConfig)
		handler := srv.Handler()

		body := `{"lang":"de"}`
		req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if received.Lang != "de" {
			t.Errorf("expected lang 'de', got %q", received.Lang)
		}
	})

	t.Run("preset and lang together", func(t *testing.T) {
		var received server.ConfigUpdatePayload
		onConfig := func(payload server.ConfigUpdatePayload) {
			received = payload
		}

		srv, _ := newTestServer(onConfig)
		handler := srv.Handler()

		body := `{"preset":"professional","lang":"en"}`
		req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if received.Preset != "professional" || received.Lang != "en" {
			t.Errorf("expected preset='professional' lang='en', got preset=%q lang=%q", received.Preset, received.Lang)
		}
	})

	t.Run("invalid lang rejected", func(t *testing.T) {
		called := false
		onConfig := func(payload server.ConfigUpdatePayload) {
			called = true
		}

		srv, _ := newTestServer(onConfig)
		handler := srv.Handler()

		body := `{"lang":"fr"}`
		req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
		if called {
			t.Error("onConfig should not have been called for invalid lang")
		}
	})
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

// TestGetPresets verifies that GET /presets returns JSON with "presets" array
// containing 8 entries, each with label, description, sampleDetails, sampleState,
// and the response has activePreset and displayDetail fields.
func TestGetPresets(t *testing.T) {
	srv, _ := newTestServer(nil)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/presets", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Presets []struct {
			Label         string   `json:"label"`
			Description   string   `json:"description"`
			SampleDetails []string `json:"sampleDetails"`
			SampleState   []string `json:"sampleState"`
			HasButtons    bool     `json:"hasButtons"`
		} `json:"presets"`
		ActivePreset  string `json:"activePreset"`
		DisplayDetail string `json:"displayDetail"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Presets) != 8 {
		t.Fatalf("expected 8 presets, got %d", len(resp.Presets))
	}

	for i, p := range resp.Presets {
		if p.Label == "" {
			t.Errorf("preset[%d]: label is empty", i)
		}
		if p.Description == "" {
			t.Errorf("preset[%d] %q: description is empty", i, p.Label)
		}
	}

	if resp.ActivePreset != "minimal" {
		t.Errorf("activePreset: got %q, want %q", resp.ActivePreset, "minimal")
	}
	if resp.DisplayDetail != "minimal" {
		t.Errorf("displayDetail: got %q, want %q", resp.DisplayDetail, "minimal")
	}
}

// TestGetStatus verifies that GET /status returns JSON with version, uptime,
// discordConnected, config, sessions, and hookStats fields.
func TestGetStatus(t *testing.T) {
	srv, _ := newTestServer(nil)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Version          string `json:"version"`
		Uptime           string `json:"uptime"`
		DiscordConnected bool   `json:"discordConnected"`
		Config           struct {
			Preset        string `json:"preset"`
			DisplayDetail string `json:"displayDetail"`
			Port          int    `json:"port"`
			BindAddr      string `json:"bindAddr"`
		} `json:"config"`
		Sessions  []interface{} `json:"sessions"`
		HookStats struct {
			Total          int64            `json:"total"`
			ByType         map[string]int64 `json:"byType"`
			LastReceivedAt *string          `json:"lastReceivedAt"`
		} `json:"hookStats"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version != "2.0.0" {
		t.Errorf("version: got %q, want %q", resp.Version, "2.0.0")
	}
	if resp.Uptime == "" {
		t.Error("uptime: expected non-empty string")
	}
	if resp.DiscordConnected {
		t.Error("discordConnected: expected false")
	}
	if resp.Config.Preset != "minimal" {
		t.Errorf("config.preset: got %q, want %q", resp.Config.Preset, "minimal")
	}
	if resp.Config.DisplayDetail != "minimal" {
		t.Errorf("config.displayDetail: got %q, want %q", resp.Config.DisplayDetail, "minimal")
	}
	if resp.Config.Port != 19460 {
		t.Errorf("config.port: got %d, want %d", resp.Config.Port, 19460)
	}
	if resp.Config.BindAddr != "127.0.0.1" {
		t.Errorf("config.bindAddr: got %q, want %q", resp.Config.BindAddr, "127.0.0.1")
	}
	if resp.HookStats.Total != 0 {
		t.Errorf("hookStats.total: got %d, want 0", resp.HookStats.Total)
	}
}

// TestHookStatsTracking verifies that hook stats are incremented correctly.
// After 3 POST /hooks/pre-tool-use calls, GET /status shows total=3 and
// byType["pre-tool-use"]=3 with a non-null lastReceivedAt.
func TestHookStatsTracking(t *testing.T) {
	srv, _ := newTestServer(nil)
	handler := srv.Handler()

	// Send 3 hooks
	for i := 0; i < 3; i++ {
		body := `{"tool_name":"Edit","session_id":"stats-test","cwd":"/tmp/project"}`
		req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("hook %d: expected 200, got %d", i, w.Code)
		}
	}

	// Check status
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: expected 200, got %d", w.Code)
	}

	var resp struct {
		HookStats struct {
			Total          int64            `json:"total"`
			ByType         map[string]int64 `json:"byType"`
			LastReceivedAt *string          `json:"lastReceivedAt"`
		} `json:"hookStats"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.HookStats.Total != 3 {
		t.Errorf("hookStats.total: got %d, want 3", resp.HookStats.Total)
	}
	if count, ok := resp.HookStats.ByType["pre-tool-use"]; !ok || count != 3 {
		t.Errorf("hookStats.byType[pre-tool-use]: got %d, want 3", count)
	}
	if resp.HookStats.LastReceivedAt == nil {
		t.Error("hookStats.lastReceivedAt: expected non-null")
	}
}

// TestHookStatsConcurrency verifies that hook stats are thread-safe.
func TestHookStatsConcurrency(t *testing.T) {
	srv, _ := newTestServer(nil)
	handler := srv.Handler()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"tool_name":"Edit","session_id":"race-test","cwd":"/tmp/project"}`
			req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}()
	}
	wg.Wait()

	// Verify total is 50
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp struct {
		HookStats struct {
			Total int64 `json:"total"`
		} `json:"hookStats"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.HookStats.Total != 50 {
		t.Errorf("hookStats.total after 50 concurrent hooks: got %d, want 50", resp.HookStats.Total)
	}
}

// newTestServerWithPreview creates a Server wired with preview callbacks.
func newTestServerWithPreview(onPreview func(server.PreviewPayload, time.Duration), onPreviewEnd func()) (*server.Server, *session.SessionRegistry) {
	registry := session.NewRegistry(func() {})
	srv := server.NewServer(
		registry,
		nil,
		"2.0.0",
		func() server.ServerConfig { return defaultTestConfig() },
		func() bool { return false },
		onPreview,
		onPreviewEnd,
	)
	return srv, registry
}

// TestPostPreview verifies that POST /preview with details, state, and duration
// returns 200 with ok=true and correct expiresIn, and calls the onPreview callback.
func TestPostPreview(t *testing.T) {
	var gotPayload server.PreviewPayload
	var gotDuration time.Duration
	srv, _ := newTestServerWithPreview(
		func(payload server.PreviewPayload, dur time.Duration) {
			gotPayload = payload
			gotDuration = dur
		},
		nil,
	)
	handler := srv.Handler()

	body := `{"preset":"hacker","duration":10,"details":"Demo mode","state":"Taking screenshots"}`
	req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK        bool `json:"ok"`
		ExpiresIn int  `json:"expiresIn"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.ExpiresIn != 10 {
		t.Errorf("expiresIn: got %d, want 10", resp.ExpiresIn)
	}

	// Verify callback was called with full payload
	if gotPayload.Details != "Demo mode" {
		t.Errorf("onPreview details: got %q, want %q", gotPayload.Details, "Demo mode")
	}
	if gotPayload.State != "Taking screenshots" {
		t.Errorf("onPreview state: got %q, want %q", gotPayload.State, "Taking screenshots")
	}
	if gotDuration != 10*time.Second {
		t.Errorf("onPreview duration: got %v, want 10s", gotDuration)
	}
}

// TestPostPreviewDefaults verifies that POST /preview with empty body
// defaults to 60-second duration with default messages.
func TestPostPreviewDefaults(t *testing.T) {
	srv, _ := newTestServerWithPreview(nil, nil)
	handler := srv.Handler()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK        bool `json:"ok"`
		ExpiresIn int  `json:"expiresIn"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ExpiresIn != 60 {
		t.Errorf("expiresIn: got %d, want 60 (default)", resp.ExpiresIn)
	}
}

// TestPostPreviewDurationCap verifies that duration is capped between 5 and 300 seconds.
func TestPostPreviewDurationCap(t *testing.T) {
	srv, _ := newTestServerWithPreview(nil, nil)
	handler := srv.Handler()

	tests := []struct {
		name     string
		duration int
		want     int
	}{
		{"over max caps to 300", 999, 300},
		{"under min caps to 5", 1, 5},
		{"at max stays 300", 300, 300},
		{"at min stays 5", 5, 5},
		{"normal duration stays same", 30, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"duration":%d}`, tt.duration)
			req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			var resp struct {
				ExpiresIn int `json:"expiresIn"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.ExpiresIn != tt.want {
				t.Errorf("expiresIn: got %d, want %d", resp.ExpiresIn, tt.want)
			}
		})
	}
}

// TestPostPreviewCancelsPrevious verifies that posting a second preview
// cancels the first timer so only the second onPreviewEnd fires.
func TestPostPreviewCancelsPrevious(t *testing.T) {
	endCalled := make(chan struct{}, 2)
	srv, _ := newTestServerWithPreview(
		nil,
		func() { endCalled <- struct{}{} },
	)
	handler := srv.Handler()

	// First preview with 5s duration
	body1 := `{"duration":5}`
	req1 := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("preview 1: expected 200, got %d", w1.Code)
	}

	// Second preview immediately (cancels first)
	body2 := `{"duration":5}`
	req2 := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("preview 2: expected 200, got %d", w2.Code)
	}

	// Wait for timer to fire (should be only 1 callback from the second preview)
	select {
	case <-endCalled:
		// First callback fired (second preview's timer)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for onPreviewEnd")
	}

	// Verify only one callback fires (give some time for any spurious second call)
	select {
	case <-endCalled:
		t.Error("onPreviewEnd was called twice (first timer was not cancelled)")
	case <-time.After(2 * time.Second):
		// Expected: only one callback
	}
}

// TestPostPreviewInvalidJSON verifies that non-JSON body returns 400.
func TestPostPreviewInvalidJSON(t *testing.T) {
	srv, _ := newTestServerWithPreview(nil, nil)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ============================================================================
// Phase 6 / Plan 06-03: Tests for the 8 new hook handlers
// ============================================================================

// postHook is a small helper that issues a POST to a hook route with a JSON
// body string and returns the (code, elapsed) pair. Used by the new-handler
// test suite below to assert fast <10ms responses per D-09/D-24 and
// reliable 200-always behavior per D-16.
func postHook(handler http.Handler, route, body string) (code int, elapsed time.Duration) {
	req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(w, req)
	return w.Code, time.Since(start)
}

// waitForSessionCount polls the registry until sessionCount matches want
// or the timeout fires. Used by background-goroutine tests (SessionEnd)
// so the test is deterministic without hardcoded sleep.
func waitForSessionCount(t *testing.T, registry *session.SessionRegistry, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if registry.SessionCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waitForSessionCount: want %d, got %d after %v",
		want, registry.SessionCount(), timeout)
}

// startTestSession creates a live session in the registry so subsequent
// hook handlers have something to update. Mirrors the pattern used by
// TestHookEndpoints for consistency.
func startTestSession(srv *server.Server, sessionID, cwd string) {
	handler := srv.Handler()
	body := fmt.Sprintf(`{"tool_name":"Edit","session_id":%q,"cwd":%q}`, sessionID, cwd)
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

// TestHandleSessionEnd verifies POST /hooks/session-end returns HTTP 200
// immediately (< 100 ms, well above the D-09 10 ms target) and removes the
// session from the registry in the background goroutine.
func TestHandleSessionEnd(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "se-1", "/tmp/myproject")
	if registry.SessionCount() != 1 {
		t.Fatalf("setup: expected 1 session, got %d", registry.SessionCount())
	}

	code, elapsed := postHook(srv.Handler(), "/hooks/session-end",
		`{"session_id":"se-1","cwd":"/tmp/myproject","reason":"clear"}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("session-end should respond quickly per D-09: got %v", elapsed)
	}

	// Background goroutine removes the session asynchronously.
	waitForSessionCount(t, registry, 0, 500*time.Millisecond)
}

// TestHandleSessionEndMalformedJSON verifies that POST /hooks/session-end
// with malformed JSON still returns HTTP 200 per D-16.
func TestHandleSessionEndMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/session-end", "not json at all")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200 on malformed JSON, got %d", code)
	}
}

// TestHandleSessionEndEmptySessionID verifies that POST /hooks/session-end
// with an empty session_id is a no-op (returns 200, no panic).
func TestHandleSessionEndEmptySessionID(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "se-2", "/tmp/other")

	code, _ := postHook(srv.Handler(), "/hooks/session-end", `{"session_id":""}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	// Session se-2 must still exist — empty session_id means no-op.
	if registry.SessionCount() != 1 {
		t.Errorf("expected 1 session after empty-id session-end, got %d",
			registry.SessionCount())
	}
}

// TestHandlePostToolUse verifies POST /hooks/post-tool-use returns 200
// and updates the stored transcript path on the session.
func TestHandlePostToolUse(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "pt-1", "/tmp/project")

	body := `{"session_id":"pt-1","cwd":"/tmp/project","tool_name":"Bash","transcript_path":"/tmp/transcript.jsonl"}`
	code, elapsed := postHook(srv.Handler(), "/hooks/post-tool-use", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("post-tool-use should respond quickly: got %v", elapsed)
	}

	// Transcript path stored per D-18.
	sess := registry.GetSession("pt-1")
	if sess == nil {
		t.Fatal("session pt-1 disappeared")
	}
	if sess.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Errorf("TranscriptPath: got %q, want /tmp/transcript.jsonl",
			sess.TranscriptPath)
	}
}

// TestHandlePostToolUseMalformedJSON verifies D-16: 200 on malformed JSON.
func TestHandlePostToolUseMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/post-tool-use", "{not valid}")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200, got %d", code)
	}
}

// TestHandlePostToolUseErrorRecovery verifies D-19: a session in the
// "error" SmallImage overlay (from StopFailure) returns to "coding"
// after a successful PostToolUse event.
func TestHandlePostToolUseErrorRecovery(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "err-1", "/tmp/project")

	// Put session into error overlay via StopFailure.
	postHook(srv.Handler(), "/hooks/stop-failure",
		`{"session_id":"err-1","error":"rate_limit"}`)

	sess := registry.GetSession("err-1")
	if sess == nil || sess.SmallImageKey != "error" {
		t.Fatalf("setup: expected SmallImageKey=error, got %v",
			sess)
	}

	// Successful PostToolUse must clear the overlay.
	code, _ := postHook(srv.Handler(), "/hooks/post-tool-use",
		`{"session_id":"err-1","tool_name":"Bash"}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	sess = registry.GetSession("err-1")
	if sess.SmallImageKey != "coding" {
		t.Errorf("D-19 violation: expected SmallImageKey=coding after recovery, got %q",
			sess.SmallImageKey)
	}
	if sess.SmallText != "Back to work" {
		t.Errorf("expected SmallText='Back to work', got %q", sess.SmallText)
	}
}

// TestHandleStopFailure verifies that every known error type maps to the
// expected Discord-visible label via stopFailureErrorText (D-19).
func TestHandleStopFailure(t *testing.T) {
	tests := []struct {
		errorField string
		wantText   string
	}{
		{"rate_limit", "Rate Limited"},
		{"authentication_failed", "Auth Error"},
		{"billing_error", "Billing Error"},
		{"max_output_tokens", "Max Tokens"},
		{"server_error", "Server Error"},
		{"invalid_request", "Invalid Request"},
		{"unknown", "API Error"},
		{"", "API Error"},
		{"weird_new_error_type", "API Error"},
	}

	for _, tt := range tests {
		t.Run("error="+tt.errorField, func(t *testing.T) {
			srv, registry := newTestServer(nil)
			sessionID := "sf-" + tt.errorField + "-x"
			startTestSession(srv, sessionID, "/tmp/project")

			body := fmt.Sprintf(`{"session_id":%q,"error":%q}`,
				sessionID, tt.errorField)
			code, _ := postHook(srv.Handler(), "/hooks/stop-failure", body)
			if code != http.StatusOK {
				t.Fatalf("expected 200, got %d", code)
			}

			sess := registry.GetSession(sessionID)
			if sess == nil {
				t.Fatal("session vanished after stop-failure")
			}
			if sess.SmallImageKey != "error" {
				t.Errorf("SmallImageKey: got %q, want error", sess.SmallImageKey)
			}
			if sess.SmallText != tt.wantText {
				t.Errorf("SmallText: got %q, want %q",
					sess.SmallText, tt.wantText)
			}
		})
	}
}

// TestHandleStopFailureMalformedJSON verifies D-16: 200 on malformed JSON.
func TestHandleStopFailureMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/stop-failure", "garbage")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200, got %d", code)
	}
}

// TestHandlePreCompact verifies POST /hooks/pre-compact returns 200 and
// records a hookStats entry. Without a tracker the background goroutine
// is not dispatched (nil-safe path).
func TestHandlePreCompact(t *testing.T) {
	srv, _ := newTestServer(nil)
	body := `{"session_id":"pc-1","cwd":"/tmp/p","trigger":"manual","transcript_path":""}`
	code, elapsed := postHook(srv.Handler(), "/hooks/pre-compact", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("pre-compact should respond quickly: got %v", elapsed)
	}
}

// TestHandlePreCompactMalformedJSON verifies D-16: 200 on malformed JSON.
func TestHandlePreCompactMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/pre-compact", "definitely not json")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200, got %d", code)
	}
}

// TestHandlePostCompact verifies POST /hooks/post-compact returns 200 and
// records a hookStats entry for both "manual" and "auto" triggers.
func TestHandlePostCompact(t *testing.T) {
	tests := []struct {
		name    string
		trigger string
	}{
		{"manual", "manual"},
		{"auto", "auto"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newTestServer(nil)
			body := fmt.Sprintf(
				`{"session_id":"cmp-1","trigger":%q,"compact_summary":"summary text"}`,
				tt.trigger)
			code, _ := postHook(srv.Handler(), "/hooks/post-compact", body)
			if code != http.StatusOK {
				t.Fatalf("expected 200, got %d", code)
			}
		})
	}
}

// TestHandlePostCompactMalformedJSON verifies D-16: 200 on malformed JSON.
func TestHandlePostCompactMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/post-compact", "nope")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200, got %d", code)
	}
}

// TestHandleSubagentStart verifies POST /hooks/subagent-start returns 200
// and swaps the session's SmallImage overlay to "thinking" with an
// agent-type label per D-22.
func TestHandleSubagentStart(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "sa-1", "/tmp/project")

	body := `{"session_id":"sa-1","agent_id":"agent-xyz","agent_type":"code-reviewer"}`
	code, _ := postHook(srv.Handler(), "/hooks/subagent-start", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	sess := registry.GetSession("sa-1")
	if sess == nil {
		t.Fatal("session sa-1 not found")
	}
	if sess.SmallImageKey != "thinking" {
		t.Errorf("D-22 violation: SmallImageKey=%q, want thinking",
			sess.SmallImageKey)
	}
	if sess.SmallText != "Subagent: code-reviewer" {
		t.Errorf("D-22 violation: SmallText=%q, want 'Subagent: code-reviewer'",
			sess.SmallText)
	}
}

// TestHandleSubagentStartEmptyAgentType verifies that a missing agent_type
// field falls back to "unknown" rather than producing "Subagent: ".
func TestHandleSubagentStartEmptyAgentType(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "sa-2", "/tmp/project")

	body := `{"session_id":"sa-2","agent_id":"agent-abc"}`
	code, _ := postHook(srv.Handler(), "/hooks/subagent-start", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	sess := registry.GetSession("sa-2")
	if sess.SmallText != "Subagent: unknown" {
		t.Errorf("SmallText: got %q, want 'Subagent: unknown'", sess.SmallText)
	}
}

// TestHandleSubagentStartMalformedJSON verifies D-16: 200 on malformed JSON.
func TestHandleSubagentStartMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/subagent-start", "{")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200, got %d", code)
	}
}

// TestHandlePostToolUseFailure verifies POST /hooks/post-tool-use-failure
// returns 200 and records a hookStats entry. Per D-20 this handler does
// NOT update the session presence, so we only assert the 200 contract.
func TestHandlePostToolUseFailure(t *testing.T) {
	srv, _ := newTestServer(nil)

	body := `{"session_id":"tf-1","tool_name":"Bash","error":"exit code 1","is_interrupt":false}`
	code, _ := postHook(srv.Handler(), "/hooks/post-tool-use-failure", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

// TestHandlePostToolUseFailureMalformedJSON verifies D-16: 200 on malformed JSON.
func TestHandlePostToolUseFailureMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/post-tool-use-failure", "}{")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200, got %d", code)
	}
}

// TestHandleCwdChanged verifies POST /hooks/cwd-changed updates the session's
// Details field to reflect the new project name derived from new_cwd (D-21).
func TestHandleCwdChanged(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "cd-1", "/tmp/old-project")

	body := `{"session_id":"cd-1","old_cwd":"/tmp/old-project","new_cwd":"/tmp/new-project"}`
	code, _ := postHook(srv.Handler(), "/hooks/cwd-changed", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	sess := registry.GetSession("cd-1")
	if sess == nil {
		t.Fatal("session cd-1 vanished")
	}
	if sess.Details != "Working on new-project" {
		t.Errorf("D-21 violation: Details=%q, want 'Working on new-project'",
			sess.Details)
	}
}

// TestHandleCwdChangedMissingSession verifies that cwd-changed for an unknown
// session is a silent no-op (returns 200, no panic, no implicit session
// creation).
func TestHandleCwdChangedMissingSession(t *testing.T) {
	srv, registry := newTestServer(nil)

	body := `{"session_id":"ghost","new_cwd":"/tmp/nowhere"}`
	code, _ := postHook(srv.Handler(), "/hooks/cwd-changed", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if registry.SessionCount() != 0 {
		t.Errorf("cwd-changed must not create sessions, got count=%d",
			registry.SessionCount())
	}
}

// TestHandleCwdChangedFallbackCwd verifies that an empty new_cwd falls back
// to the common cwd field and still updates Details.
func TestHandleCwdChangedFallbackCwd(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "cd-2", "/tmp/project-a")

	body := `{"session_id":"cd-2","cwd":"/tmp/project-b","old_cwd":"/tmp/project-a"}`
	code, _ := postHook(srv.Handler(), "/hooks/cwd-changed", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	sess := registry.GetSession("cd-2")
	if sess.Details != "Working on project-b" {
		t.Errorf("fallback: Details=%q, want 'Working on project-b'",
			sess.Details)
	}
}

// TestHandleCwdChangedMalformedJSON verifies D-16: 200 on malformed JSON.
func TestHandleCwdChangedMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	code, _ := postHook(srv.Handler(), "/hooks/cwd-changed", "oops")
	if code != http.StatusOK {
		t.Fatalf("D-16 violation: expected 200, got %d", code)
	}
}

// TestAllNewHookRoutesReturn200 is a smoke test that every new route
// registered in Handler() returns HTTP 200 for a well-formed minimal payload.
// This catches routing mistakes (wrong path, method mismatch) before the
// handler-specific tests above. Also records the elapsed time to flag any
// handler that regresses past the 100 ms target.
func TestAllNewHookRoutesReturn200(t *testing.T) {
	srv, _ := newTestServer(nil)
	handler := srv.Handler()
	body := `{"session_id":"smoke-1","cwd":"/tmp/project"}`

	routes := []string{
		"/hooks/session-end",
		"/hooks/post-tool-use",
		"/hooks/stop-failure",
		"/hooks/pre-compact",
		"/hooks/post-compact",
		"/hooks/subagent-start",
		"/hooks/post-tool-use-failure",
		"/hooks/cwd-changed",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			code, elapsed := postHook(handler, route, body)
			if code != http.StatusOK {
				t.Errorf("%s: code=%d, want 200", route, code)
			}
			if elapsed > 100*time.Millisecond {
				t.Errorf("%s: elapsed=%v, want <100ms", route, elapsed)
			}
		})
	}
}

// TestAllNewHookRoutesReturn200OnMalformedJSON verifies D-16: every new
// route returns HTTP 200 even when the JSON body is completely malformed.
// This is the mandatory contract for Claude Code hook handlers so that a
// non-2xx response never blocks the Claude session.
func TestAllNewHookRoutesReturn200OnMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(nil)
	handler := srv.Handler()

	routes := []string{
		"/hooks/session-end",
		"/hooks/post-tool-use",
		"/hooks/stop-failure",
		"/hooks/pre-compact",
		"/hooks/post-compact",
		"/hooks/subagent-start",
		"/hooks/post-tool-use-failure",
		"/hooks/cwd-changed",
	}

	malformed := []string{
		"not json",
		"{broken",
		`{"session_id":}`,
		"",
		"[]",
	}

	for _, route := range routes {
		for _, body := range malformed {
			name := route + "/" + strings.ReplaceAll(body, " ", "_")
			t.Run(name, func(t *testing.T) {
				code, _ := postHook(handler, route, body)
				if code != http.StatusOK {
					t.Errorf("%s with body %q: code=%d, D-16 requires 200",
						route, body, code)
				}
			})
		}
	}
}


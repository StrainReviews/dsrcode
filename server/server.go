// Package server provides the HTTP server that receives Claude Code hook events
// and statusline data, translating them into session activity updates.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/StrainReviews/dsrcode/analytics"
	"github.com/StrainReviews/dsrcode/config"
	"github.com/StrainReviews/dsrcode/preset"
	"github.com/StrainReviews/dsrcode/resolver"
	"github.com/StrainReviews/dsrcode/session"
)

// ActivityMapping maps tool names to (SmallImageKey, SmallText) per D-07.
// Used by MapHookToActivity to translate Claude Code hook events into
// Discord-visible activity icons and descriptions.
var ActivityMapping = map[string]struct{ Icon, Text string }{
	"Edit":      {"coding", "Editing a file"},
	"Write":     {"coding", "Writing a file"},
	"Bash":      {"terminal", "Running a command"},
	"Read":      {"reading", "Reading a file"},
	"Grep":      {"searching", "Searching codebase"},
	"Glob":      {"searching", "Searching files"},
	"WebSearch": {"searching", "Searching the web"},
	"WebFetch":  {"searching", "Fetching web content"},
	"Task":      {"thinking", "Running a subtask"},
}

// HookPayload is the JSON body from Claude Code HTTP hooks.
// Extended per D-18 to include tool_input, hook metadata, and event-specific fields.
type HookPayload struct {
	SessionID        string          `json:"session_id"`
	Cwd              string          `json:"cwd"`
	ToolName         string          `json:"tool_name,omitempty"`
	ToolInput        json.RawMessage `json:"tool_input,omitempty"`
	HookEventName    string          `json:"hook_event_name,omitempty"`
	TranscriptPath   string          `json:"transcript_path,omitempty"`
	PermissionMode   string          `json:"permission_mode,omitempty"`
	Prompt           string          `json:"prompt,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	NotificationType string          `json:"notification_type,omitempty"`
	Message          string          `json:"message,omitempty"`
	Title            string          `json:"title,omitempty"`
}

// fileToolInput holds the parsed file_path from Edit/Write/Read tool_input.
type fileToolInput struct {
	FilePath string `json:"file_path"`
}

// bashToolInput holds the parsed command from Bash tool_input.
type bashToolInput struct {
	Command string `json:"command"`
}

// searchToolInput holds the parsed pattern from Grep/Glob tool_input.
type searchToolInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

// ExtractToolContext parses tool_input to extract file, filePath, command, and query
// based on the tool type. Returns empty strings for unknown or internal-only tools.
// Exported for test access.
func ExtractToolContext(toolName string, rawInput json.RawMessage) (file, filePath, command, query string) {
	if len(rawInput) == 0 {
		return
	}
	switch toolName {
	case "Edit", "Write", "Read":
		var input fileToolInput
		if json.Unmarshal(rawInput, &input) == nil && input.FilePath != "" {
			filePath = filepath.ToSlash(input.FilePath)
			file = filepath.Base(filePath)
		}
	case "Bash":
		var input bashToolInput
		if json.Unmarshal(rawInput, &input) == nil {
			command = input.Command
		}
	case "Grep", "Glob":
		var input searchToolInput
		if json.Unmarshal(rawInput, &input) == nil {
			query = input.Pattern
		}
	case "Task", "Agent":
		// Internal only per D-19 -- do not expose prompt to Discord
	}
	return
}

// StatuslinePayload matches Claude Code's statusline JSON structure.
type StatuslinePayload struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Model     struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"model"`
	TotalTokens  int64   `json:"total_tokens"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// ConfigUpdatePayload for POST /config endpoint per D-50.
type ConfigUpdatePayload struct {
	Preset string `json:"preset,omitempty"`
	Lang   string `json:"lang,omitempty"`
}

// HookStats tracks hook invocation statistics using atomic operations
// for thread-safe concurrent access.
type HookStats struct {
	Total          atomic.Int64              `json:"-"`
	ByType         sync.Map                  `json:"-"` // map[string]*atomic.Int64
	LastReceivedAt atomic.Pointer[time.Time] `json:"-"`
}

// hookStatsJSON is the JSON serialization format for HookStats.
type hookStatsJSON struct {
	Total          int64            `json:"total"`
	ByType         map[string]int64 `json:"byType"`
	LastReceivedAt *time.Time       `json:"lastReceivedAt"`
}

// record increments hook statistics for the given hook type.
func (h *HookStats) record(hookType string) {
	h.Total.Add(1)
	now := time.Now()
	h.LastReceivedAt.Store(&now)
	actual, _ := h.ByType.LoadOrStore(hookType, &atomic.Int64{})
	actual.(*atomic.Int64).Add(1)
}

// toJSON converts HookStats to a JSON-serializable format.
func (h *HookStats) toJSON() hookStatsJSON {
	result := hookStatsJSON{
		Total:          h.Total.Load(),
		ByType:         make(map[string]int64),
		LastReceivedAt: h.LastReceivedAt.Load(),
	}
	h.ByType.Range(func(key, value any) bool {
		result.ByType[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})
	return result
}

// ServerConfig holds the subset of configuration exposed via the status endpoint.
type ServerConfig struct {
	Preset        string `json:"preset"`
	DisplayDetail string `json:"displayDetail"`
	Port          int    `json:"port"`
	BindAddr      string `json:"bindAddr"`
	Lang          string `json:"lang"`
}

// FakeSession represents a simulated session for multi-session demo preview.
// Used to generate realistic multi-project screenshots per D-07.
type FakeSession struct {
	ProjectName string  `json:"projectName"`
	Model       string  `json:"model,omitempty"`
	Branch      string  `json:"branch,omitempty"`
	TotalTokens int64   `json:"totalTokens,omitempty"`
	TotalCost   float64 `json:"totalCost,omitempty"`
	Activity    string  `json:"activity,omitempty"` // SmallImageKey (coding, terminal, etc.)
	Status      string  `json:"status,omitempty"`   // active/idle
}

// ButtonDef is a button label+URL pair for preview per D-07.
type ButtonDef struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// PreviewPayload is the JSON body for POST /preview requests.
// Extended per D-07 with full Discord Activity control for demo screenshots.
type PreviewPayload struct {
	Preset         string        `json:"preset,omitempty"`
	DisplayDetail  string        `json:"displayDetail,omitempty"`
	Duration       int           `json:"duration,omitempty"` // seconds, default 60, min 5, max 300
	Details        string        `json:"details,omitempty"`
	State          string        `json:"state,omitempty"`
	SmallImage     string        `json:"smallImage,omitempty"`     // per D-07
	SmallText      string        `json:"smallText,omitempty"`      // per D-07
	LargeText      string        `json:"largeText,omitempty"`      // per D-07
	Buttons        []ButtonDef   `json:"buttons,omitempty"`        // per D-07
	StartTimestamp int64         `json:"startTimestamp,omitempty"` // Unix epoch, per D-07
	SessionCount   int           `json:"sessionCount,omitempty"`   // per D-07
	FakeSessions   []FakeSession `json:"fakeSessions,omitempty"`   // per D-07
}

// PreviewState tracks an active preview session with its expiration timer.
type PreviewState struct {
	Active    bool
	ExpiresAt time.Time
	Timer     *time.Timer
}

// Server manages the HTTP server for hook events.
type Server struct {
	registry         *session.SessionRegistry
	tracker          *analytics.Tracker
	onConfig         func(ConfigUpdatePayload)
	httpServer       *http.Server
	startTime        time.Time
	hookStats        HookStats
	configGetter     func() ServerConfig
	version          string
	discordConnected func() bool
	previewMu        sync.Mutex
	previewState     *PreviewState
	onPreview        func(PreviewPayload, time.Duration)
	onPreviewEnd     func()

	// lastTranscriptRead throttles PostToolUse-triggered JSONL reads per D-02:
	// max 1 read per 10 seconds per session. Keys are session IDs, values are
	// time.Time of the last read. Must not be copied.
	lastTranscriptRead sync.Map

	// onAutoExit is called when SessionEnd causes the session count to reach 0.
	// Plan 06-04 wires this callback to trigger the grace-period auto-exit goroutine.
	// For Plan 06-03 it remains nil (hook point only).
	onAutoExit func()

	// onAnalyticsSync is called with the session ID after the tracker's
	// per-session token state has been updated (from PostToolUse, SessionEnd,
	// or PreCompact handlers). Plan 06-05 wires this callback to push
	// tracker state into the session registry so the presence resolver can
	// display current token/cost/compaction/tool data. Nil = skip (safe
	// no-op, matches SetOnAutoExit precedent).
	onAnalyticsSync func(sessionID string)
}

// NewServer creates a new Server.
// registry: session registry for activity updates
// onConfig: callback for config change requests (may be nil)
// version: application version string
// configGetter: returns current config snapshot (may be nil)
// discordConnected: returns Discord connection status (may be nil)
// onPreview: callback when preview mode is activated with full payload (may be nil)
// onPreviewEnd: callback when preview mode expires (may be nil)
func NewServer(registry *session.SessionRegistry, onConfig func(ConfigUpdatePayload), version string, configGetter func() ServerConfig, discordConnected func() bool, onPreview func(PreviewPayload, time.Duration), onPreviewEnd func()) *Server {
	return &Server{
		registry:         registry,
		onConfig:         onConfig,
		startTime:        time.Now(),
		version:          version,
		configGetter:     configGetter,
		discordConnected: discordConnected,
		onPreview:        onPreview,
		onPreviewEnd:     onPreviewEnd,
	}
}

// SetTracker sets the analytics tracker for the server. Must be called before
// Start. The tracker receives tool usage, subagent spawn/complete events, and
// provides analytics data for the /status endpoint.
func (s *Server) SetTracker(t *analytics.Tracker) {
	s.tracker = t
}

// SetOnAutoExit wires the auto-exit callback invoked by handleSessionEnd when
// SessionCount() reaches 0 after removing a session. Plan 06-04 wires this to
// the grace-period auto-exit goroutine in main.go for dual-trigger shutdown
// (SessionEnd hook path + stale detection path, per D-04). Must be called
// before Start.
func (s *Server) SetOnAutoExit(f func()) {
	s.onAutoExit = f
}

// SetOnAnalyticsSync wires the analytics-to-registry sync callback invoked
// from PostToolUse, SessionEnd, and PreCompact handlers after token updates
// are written to the tracker. Plan 06-05 wires this to
// syncAnalyticsToRegistry in main.go so tracker state (tokens, cost,
// compactions, tool counts, subagents) flows back to the session registry
// for the presence resolver. Must be called before Start. Nil = skip (safe
// no-op, matches SetOnAutoExit precedent). Per the Go memory model, the
// setter write happens-before srv.Start's goroutine launch, so all HTTP
// handler goroutines observe the installed callback without a mutex.
func (s *Server) SetOnAnalyticsSync(f func(sessionID string)) {
	s.onAnalyticsSync = f
}

// MapHookToActivity maps a hookType and toolName to a (SmallImageKey, SmallText) pair.
// Exported so tests can verify the mapping directly.
func MapHookToActivity(hookType string, toolName string) (icon string, text string) {
	switch hookType {
	case "pre-tool-use":
		if mapping, ok := ActivityMapping[toolName]; ok {
			return mapping.Icon, mapping.Text
		}
		return "thinking", "Processing..."
	case "user-prompt-submit":
		return "thinking", "Thinking..."
	case "stop":
		return "idle", "Finished"
	case "notification":
		return "idle", "Waiting for input"
	default:
		return "thinking", "Processing..."
	}
}

// Handler returns the http.Handler with all routes registered.
// Uses Go 1.22+ ServeMux method routing patterns.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Dedicated hook handlers MUST be registered BEFORE the wildcard route
	// so that Go's method routing picks the specific handler first.
	mux.HandleFunc("POST /hooks/subagent-stop", s.handleSubagentStop)
	mux.HandleFunc("POST /hooks/session-end", s.handleSessionEnd)
	mux.HandleFunc("POST /hooks/post-tool-use", s.handlePostToolUse)
	mux.HandleFunc("POST /hooks/stop-failure", s.handleStopFailure)
	mux.HandleFunc("POST /hooks/pre-compact", s.handlePreCompact)
	mux.HandleFunc("POST /hooks/post-compact", s.handlePostCompact)
	mux.HandleFunc("POST /hooks/subagent-start", s.handleSubagentStart)
	mux.HandleFunc("POST /hooks/post-tool-use-failure", s.handlePostToolUseFailure)
	mux.HandleFunc("POST /hooks/cwd-changed", s.handleCwdChanged)
	mux.HandleFunc("POST /hooks/{hookType}", s.handleHook)
	mux.HandleFunc("POST /statusline", s.handleStatusline)
	mux.HandleFunc("POST /config", s.handleConfigUpdate)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /sessions", s.handleSessions)
	mux.HandleFunc("GET /presets", s.handleGetPresets)
	mux.HandleFunc("GET /status", s.handleGetStatus)
	mux.HandleFunc("POST /preview", s.handlePostPreview)
	mux.HandleFunc("GET /preview/messages", s.handleGetPreviewMessages)

	return mux
}

// handleHook processes POST /hooks/{hookType} requests from Claude Code.
// It maps the hook event to an activity and updates the session registry.
func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	hookType := r.PathValue("hookType")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var payload HookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		// Retry with sanitized JSON: Claude Code on Windows may send
		// unescaped backslashes in paths (known bug anthropics/claude-code#20015).
		sanitized := sanitizeWindowsJSON(body)
		if err2 := json.Unmarshal(sanitized, &payload); err2 != nil {
			slog.Debug("hook: invalid JSON body", "error", err2)
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
	}

	// Session ID fallback per D-31: empty session_id creates synthetic ID
	// instead of HTTP 400 rejection (fixes silent-rejection bug)
	if payload.SessionID == "" {
		projectName := filepath.Base(payload.Cwd)
		if projectName == "" || projectName == "." {
			projectName = "unknown"
		}
		payload.SessionID = "http-" + projectName
		slog.Warn("hook: empty session_id, using synthetic ID", "syntheticId", payload.SessionID, "cwd", payload.Cwd)
	}

	icon, text := MapHookToActivity(hookType, payload.ToolName)

	// Derive Details from the project name
	details := filepath.Base(payload.Cwd)
	if details == "" || details == "." {
		details = "Unknown Project"
	}

	// Parse tool_input for file/command/query context per D-19
	file, filePath, command, query := ExtractToolContext(payload.ToolName, payload.ToolInput)

	req := session.ActivityRequest{
		SessionID:     payload.SessionID,
		Cwd:           payload.Cwd,
		ToolName:      payload.ToolName,
		SmallImageKey: icon,
		SmallText:     text,
		Details:       fmt.Sprintf("Working on %s", details),
		LastFile:      file,
		LastFilePath:  filePath,
		LastCommand:   command,
		LastQuery:     query,
	}

	// Get PID from header or approximate with parent PID
	pid := 0
	if pidHeader := r.Header.Get("X-Claude-PID"); pidHeader != "" {
		if parsed, err := strconv.Atoi(pidHeader); err == nil {
			pid = parsed
		}
	}
	if pid == 0 {
		pid = os.Getppid()
	}

	// Ensure session exists before updating activity
	existing := s.registry.GetSession(payload.SessionID)
	if existing == nil {
		s.registry.StartSession(req, pid)
	}

	s.registry.UpdateActivity(payload.SessionID, req)

	// Track tool usage in analytics per D-01
	if s.tracker != nil {
		s.tracker.RecordTool(payload.SessionID, payload.ToolName)
	}

	// Agent tool_name = subagent spawn per D-01
	if payload.ToolName == "Agent" {
		var agentInput struct {
			Description  string `json:"description"`
			SubagentType string `json:"subagent_type"`
			Prompt       string `json:"prompt"`
		}
		if err := json.Unmarshal(payload.ToolInput, &agentInput); err == nil {
			name := agentInput.Description
			if name == "" {
				name = agentInput.SubagentType
			}
			if name == "" && len(agentInput.Prompt) > 0 {
				name = truncate(agentInput.Prompt, 57)
			}
			if name == "" {
				name = "Subagent"
			}
			id := fmt.Sprintf("%s-%d", payload.SessionID, time.Now().UnixNano())
			if s.tracker != nil {
				s.tracker.RecordSubagentSpawn(payload.SessionID, analytics.SubagentEntry{
					ID:          id,
					Type:        agentInput.SubagentType,
					Description: name,
					Prompt:      agentInput.Prompt,
					SpawnTime:   time.Now(),
				})
			}
		}
	}

	// Store transcript_path per D-13/D-18
	if payload.TranscriptPath != "" {
		s.registry.UpdateTranscriptPath(payload.SessionID, payload.TranscriptPath)
	}

	s.hookStats.record(hookType)

	slog.Debug("hook received", "hookType", hookType, "tool", payload.ToolName, "session", payload.SessionID)

	w.WriteHeader(http.StatusOK)
}

// postToolUseThrottleInterval is the minimum time between ParseTranscript
// reads for a single session per D-02. This bounds file I/O when Claude Code
// spams tool calls in quick succession.
const postToolUseThrottleInterval = 10 * time.Second

// StopFailurePayload is the JSON body from Claude Code StopFailure hook events (D-19).
// Per Claude Code hooks docs: fires when an API error prevents Claude from responding.
// Known error values include rate_limit, authentication_failed, billing_error,
// invalid_request, server_error, max_output_tokens, unknown. The "error" field
// may also contain a free-form message like "API Error: Rate limit reached".
type StopFailurePayload struct {
	SessionID            string `json:"session_id"`
	Cwd                  string `json:"cwd"`
	TranscriptPath       string `json:"transcript_path"`
	HookEventName        string `json:"hook_event_name,omitempty"`
	Error                string `json:"error"`
	ErrorDetails         string `json:"error_details,omitempty"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
}

// handleSessionEnd processes POST /hooks/session-end requests from Claude Code (D-09).
// Returns HTTP 200 immediately (<10ms) per D-16/D-24, then performs a background
// final JSONL read via analytics.ParseTranscript and removes the session from
// the registry. Plan 06-04 will wire onAutoExit to trigger the grace-period exit.
func (s *Server) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	// Always respond with HTTP 200 per D-16 (non-blocking for Claude Code).
	w.WriteHeader(http.StatusOK)

	var payload HookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("session-end: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("session-end: empty session_id, ignoring")
		return
	}

	reason := payload.Reason
	transcriptPath := payload.TranscriptPath
	sessionID := payload.SessionID

	slog.Info("session-end received", "session", sessionID, "reason", reason)
	s.hookStats.record("session-end")

	// Background goroutine: final JSONL read (if transcript_path present),
	// then remove the session from the registry. This cannot block the HTTP
	// response per D-09. Wrapped in recover so a panic does not crash the daemon.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("session-end background panic", "session", sessionID, "panic", r)
			}
		}()

		if s.tracker != nil && transcriptPath != "" {
			result, err := analytics.ParseTranscript(transcriptPath)
			if err != nil {
				slog.Debug("session-end: final transcript parse failed",
					"session", sessionID, "error", err)
			} else {
				for model, tokens := range result.Tokens {
					s.tracker.UpdateTokens(sessionID, model, tokens)
				}
			}
		}

		// Plan 06-05: push the final tracker state into the session registry
		// BEFORE EndSession so the presence resolver can reflect the final
		// token/cost/compaction/tool counts in one last update before the
		// session is removed. Safe no-op when the callback is not wired.
		if s.onAnalyticsSync != nil {
			s.onAnalyticsSync(sessionID)
		}

		s.registry.EndSession(sessionID)

		// Plan 06-04 hook point: if onAutoExit is wired and refcount hit 0,
		// fire the grace-period shutdown. For Plan 06-03 this remains nil.
		if s.onAutoExit != nil && s.registry.SessionCount() == 0 {
			s.onAutoExit()
		}
	}()
}

// handlePostToolUse processes POST /hooks/post-tool-use requests from Claude Code (D-02).
// Returns HTTP 200 immediately and triggers a throttled JSONL read (max 1 per 10s
// per session) in a background goroutine. Also clears the error overlay if the
// session was previously in StopFailure state per D-19.
func (s *Server) handlePostToolUse(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var payload HookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("post-tool-use: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("post-tool-use: empty session_id, ignoring")
		return
	}

	sessionID := payload.SessionID
	transcriptPath := payload.TranscriptPath

	// Record tool usage in analytics.
	if s.tracker != nil && payload.ToolName != "" {
		s.tracker.RecordTool(sessionID, payload.ToolName)
	}

	// Throttled background JSONL read per D-02: max 1 read per 10 seconds per session.
	// The sync.Map stores the last read timestamp; we skip if we are still inside
	// the throttle window. Otherwise we record "now" and dispatch the background read.
	if s.tracker != nil && transcriptPath != "" {
		now := time.Now()
		shouldRead := true
		if lastVal, ok := s.lastTranscriptRead.Load(sessionID); ok {
			if lastTime, ok := lastVal.(time.Time); ok {
				if now.Sub(lastTime) < postToolUseThrottleInterval {
					shouldRead = false
				}
			}
		}
		if shouldRead {
			s.lastTranscriptRead.Store(sessionID, now)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("post-tool-use background panic",
							"session", sessionID, "panic", r)
					}
				}()

				result, err := analytics.ParseTranscript(transcriptPath)
				if err != nil {
					slog.Debug("post-tool-use: transcript parse failed",
						"session", sessionID, "error", err)
					return
				}
				for model, tokens := range result.Tokens {
					s.tracker.UpdateTokens(sessionID, model, tokens)
				}

				// Plan 06-05: sync updated tracker state (tokens, cost,
				// tool counts) into the session registry so the presence
				// resolver can reflect the new token totals on the next
				// debounced update. Bounded by the D-02 10s throttle
				// above, so this cannot thrash registry mutations.
				if s.onAnalyticsSync != nil {
					s.onAnalyticsSync(sessionID)
				}
			}()
		}
	}

	// Clear the error overlay if the session was in StopFailure state per D-19:
	// a successful PostToolUse means the API is responding again.
	if existing := s.registry.GetSession(sessionID); existing != nil && existing.SmallImageKey == "error" {
		s.registry.UpdateActivity(sessionID, session.ActivityRequest{
			SessionID:     sessionID,
			SmallImageKey: "coding",
			SmallText:     "Back to work",
		})
	}

	// Keep the transcript path in sync for later reads (D-18).
	if transcriptPath != "" {
		s.registry.UpdateTranscriptPath(sessionID, transcriptPath)
	}

	s.hookStats.record("post-tool-use")
	slog.Debug("post-tool-use received",
		"session", sessionID, "tool", payload.ToolName)
}

// stopFailureErrorText maps the Claude Code StopFailure "error" field to a
// concise Discord-visible label. Returns a default ("API Error") when the
// error value is empty or unrecognized, so the overlay always shows something.
func stopFailureErrorText(errorField string) string {
	switch errorField {
	case "rate_limit":
		return "Rate Limited"
	case "authentication_failed":
		return "Auth Error"
	case "billing_error":
		return "Billing Error"
	case "max_output_tokens":
		return "Max Tokens"
	case "server_error":
		return "Server Error"
	case "invalid_request":
		return "Invalid Request"
	case "unknown", "":
		return "API Error"
	default:
		return "API Error"
	}
}

// handleStopFailure processes POST /hooks/stop-failure requests from Claude Code (D-19).
// Swaps the session's SmallImage to the "error" status overlay and updates
// SmallText with a concise error label. The overlay is cleared by the next
// successful PostToolUse/Stop per D-19. State/Details are left untouched so
// token/model info remains visible while the error is shown.
func (s *Server) handleStopFailure(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var payload StopFailurePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("stop-failure: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("stop-failure: empty session_id, ignoring")
		return
	}

	errorText := stopFailureErrorText(payload.Error)

	s.registry.UpdateActivity(payload.SessionID, session.ActivityRequest{
		SessionID:     payload.SessionID,
		SmallImageKey: "error",
		SmallText:     errorText,
	})

	slog.Warn("stop-failure received",
		"session", payload.SessionID,
		"error", payload.Error,
		"details", payload.ErrorDetails)
	s.hookStats.record("stop-failure")
}

// PreCompactPayload is the JSON body from Claude Code PreCompact hook events (D-15).
// Fires BEFORE compaction so the daemon can snapshot the current token totals
// (via analytics.ParseTranscript) as a baseline for compaction verification.
// Per Claude Code hooks docs: trigger is "manual" or "auto".
type PreCompactPayload struct {
	SessionID          string `json:"session_id"`
	Cwd                string `json:"cwd"`
	TranscriptPath     string `json:"transcript_path"`
	HookEventName      string `json:"hook_event_name,omitempty"`
	Trigger            string `json:"trigger,omitempty"` // "manual" or "auto"
	CustomInstructions string `json:"custom_instructions,omitempty"`
}

// handlePreCompact processes POST /hooks/pre-compact requests from Claude Code (D-15).
// Returns HTTP 200 immediately per D-16, then (in a background goroutine with
// panic recovery) reads the transcript via analytics.ParseTranscript to save
// the token baseline before compaction. PostCompact verifies the baseline.
//
// Unlike PostToolUse, PreCompact is NOT throttled: compaction is infrequent
// so a single ParseTranscript read per event is acceptable file I/O.
func (s *Server) handlePreCompact(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var payload PreCompactPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("pre-compact: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("pre-compact: empty session_id, ignoring")
		return
	}

	sessionID := payload.SessionID
	transcriptPath := payload.TranscriptPath
	trigger := payload.Trigger

	slog.Info("pre-compact received",
		"session", sessionID, "trigger", trigger)
	s.hookStats.record("pre-compact")

	// Background goroutine: snapshot token baseline via ParseTranscript.
	// Cannot block the HTTP response per D-09 pattern; wrapped in recover so
	// a panic in analytics.ParseTranscript does not crash the daemon.
	if s.tracker != nil && transcriptPath != "" {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("pre-compact background panic",
						"session", sessionID, "panic", r)
				}
			}()

			result, err := analytics.ParseTranscript(transcriptPath)
			if err != nil {
				slog.Debug("pre-compact: transcript parse failed",
					"session", sessionID, "error", err)
				return
			}
			for model, tokens := range result.Tokens {
				s.tracker.UpdateTokens(sessionID, model, tokens)
			}
			slog.Info("pre-compact baseline saved",
				"session", sessionID, "models", len(result.Tokens))

			// Plan 06-05: sync baseline snapshot state (tokens, baselines,
			// compaction count once PostCompact follows) into the session
			// registry so the presence resolver reflects the new baseline.
			if s.onAnalyticsSync != nil {
				s.onAnalyticsSync(sessionID)
			}
		}()
	}
}

// PostCompactPayload is the JSON body from Claude Code PostCompact hook events (D-23).
// Fires AFTER compaction with the trigger and the compacted conversation summary.
// Per canonical_refs (Issue #40492 open): payload does NOT include pre/post
// token counts — verification relies on the PreCompact baseline set via
// analytics.ParseTranscript.
type PostCompactPayload struct {
	SessionID      string `json:"session_id"`
	Cwd            string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
	HookEventName  string `json:"hook_event_name,omitempty"`
	Trigger        string `json:"trigger,omitempty"` // "manual" or "auto"
	CompactSummary string `json:"compact_summary,omitempty"`
}

// handlePostCompact processes POST /hooks/post-compact requests from Claude Code (D-23).
// Returns HTTP 200 per D-16, records the compaction event in analytics, and
// logs the compact_summary length for observability. Uses a synthetic UUID
// derived from trigger + unix-nano so CompactionTracker's UUID dedup still
// treats each event as distinct (Claude Code payload has no true UUID per
// Issue #40492).
func (s *Server) handlePostCompact(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var payload PostCompactPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("post-compact: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("post-compact: empty session_id, ignoring")
		return
	}

	// Increment compaction counter per D-23. Synthetic UUID keeps
	// CompactionTracker's built-in dedup happy (each event is unique).
	if s.tracker != nil {
		uuid := fmt.Sprintf("compact-%s-%d", payload.Trigger, time.Now().UnixNano())
		s.tracker.RecordCompaction(payload.SessionID, uuid)
	}

	slog.Info("post-compact received",
		"session", payload.SessionID,
		"trigger", payload.Trigger,
		"summaryLen", len(payload.CompactSummary))
	s.hookStats.record("post-compact")
}

// SubagentStartPayload is the JSON body from Claude Code SubagentStart hook events (D-22).
// Fires when a subagent is spawned via the Agent tool. Added in Claude Code
// v2.0.43, agent_id + agent_type added in v2.1.69. Per canonical_refs: the
// payload does NOT include prompt or description (Issue #32016 open).
type SubagentStartPayload struct {
	SessionID      string `json:"session_id"`
	Cwd            string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
	HookEventName  string `json:"hook_event_name,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
}

// handleSubagentStart processes POST /hooks/subagent-start requests from Claude Code (D-22).
// Returns HTTP 200 per D-16, spawns the subagent in analytics.Tracker so it
// appears in SubagentTree immediately (not just at first tool use), and swaps
// the session presence to the "thinking" overlay showing the agent type.
//
// If the session does not yet exist in the registry, the presence update is
// a no-op — the subagent will still be tracked in analytics. SubagentStart
// itself must NOT implicitly create a session: that would hide the Agent
// tool's parent-session creation path.
func (s *Server) handleSubagentStart(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var payload SubagentStartPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("subagent-start: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("subagent-start: empty session_id, ignoring")
		return
	}

	// Record subagent spawn in analytics per D-22. Without a prompt/description
	// we fall back to agent_type for the Description field so the SubagentTree
	// UI still has something human-readable.
	if s.tracker != nil {
		description := payload.AgentType
		if description == "" {
			description = "Subagent"
		}
		s.tracker.RecordSubagentSpawn(payload.SessionID, analytics.SubagentEntry{
			ID:          payload.AgentID,
			Type:        payload.AgentType,
			Description: description,
			SpawnTime:   time.Now(),
		})
	}

	// Swap presence to thinking overlay with agent_type label per D-22.
	agentLabel := payload.AgentType
	if agentLabel == "" {
		agentLabel = "unknown"
	}
	s.registry.UpdateActivity(payload.SessionID, session.ActivityRequest{
		SessionID:     payload.SessionID,
		SmallImageKey: "thinking",
		SmallText:     fmt.Sprintf("Subagent: %s", agentLabel),
	})

	s.hookStats.record("subagent-start")
	slog.Debug("subagent-start received",
		"session", payload.SessionID,
		"agentId", payload.AgentID,
		"agentType", payload.AgentType)
}

// PostToolUseFailurePayload is the JSON body from Claude Code PostToolUseFailure
// hook events (D-20). Fires when a tool call fails (non-zero exit, permission
// denial, or user interrupt). We only use the tool_name + error + is_interrupt
// fields for counter tracking — other fields like tool_input and tool_use_id
// are ignored.
type PostToolUseFailurePayload struct {
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	HookEventName string `json:"hook_event_name,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	Error         string `json:"error,omitempty"`
	IsInterrupt   bool   `json:"is_interrupt,omitempty"`
}

// handlePostToolUseFailure processes POST /hooks/post-tool-use-failure requests
// from Claude Code (D-20). Returns HTTP 200 per D-16, increments an analytics
// error counter via tracker.RecordTool("{tool}_error"), and logs the error.
// Per D-20 this handler does NOT update the session presence (too granular);
// the user sees the error via StopFailure when the turn completes.
func (s *Server) handlePostToolUseFailure(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var payload PostToolUseFailurePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("post-tool-use-failure: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("post-tool-use-failure: empty session_id, ignoring")
		return
	}

	// Record error counter via tracker.RecordTool("{tool}_error") per D-20.
	// The suffix keeps failures distinct from successful tool calls in
	// analytics while reusing the same counter infrastructure.
	if s.tracker != nil && payload.ToolName != "" {
		s.tracker.RecordTool(payload.SessionID, payload.ToolName+"_error")
	}

	s.hookStats.record("post-tool-use-failure")
	slog.Debug("post-tool-use-failure received",
		"session", payload.SessionID,
		"tool", payload.ToolName,
		"error", payload.Error,
		"interrupt", payload.IsInterrupt)
}

// CwdChangedPayload is the JSON body from Claude Code CwdChanged hook events (D-21).
// Fires when the working directory changes (e.g., via cd or Claude switching
// context). Per canonical_refs: no matcher support, always fires.
type CwdChangedPayload struct {
	SessionID      string `json:"session_id"`
	Cwd            string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
	HookEventName  string `json:"hook_event_name,omitempty"`
	OldCwd         string `json:"old_cwd,omitempty"`
	NewCwd         string `json:"new_cwd,omitempty"`
}

// handleCwdChanged processes POST /hooks/cwd-changed requests from Claude Code (D-21).
// Returns HTTP 200 per D-16. If the session exists, updates its Details field
// to reflect the new project derived from filepath.Base(new_cwd). The session
// is NOT implicitly created — CwdChanged after an unknown session is logged
// and ignored so the Registry's project lineage stays clean.
//
// Plan 06-05 integration point: a full-project-context swap (updating the
// Cwd, ProjectName, ProjectPath fields, not just Details) requires a new
// Registry.UpdateProjectContext method. For Plan 06-03 we only update the
// visible Details string — the underlying ProjectName stays tied to the
// session's original cwd.
func (s *Server) handleCwdChanged(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var payload CwdChangedPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("cwd-changed: invalid JSON body", "error", err)
		return
	}
	if payload.SessionID == "" {
		slog.Debug("cwd-changed: empty session_id, ignoring")
		return
	}

	// Session must exist for update (D-21 does not create sessions).
	existing := s.registry.GetSession(payload.SessionID)
	if existing == nil {
		slog.Debug("cwd-changed: session not found, ignoring",
			"session", payload.SessionID)
		s.hookStats.record("cwd-changed")
		return
	}

	// Derive new project label from new_cwd. Fall back to cwd (top-level
	// common input field) if new_cwd is empty.
	newCwd := payload.NewCwd
	if newCwd == "" {
		newCwd = payload.Cwd
	}
	newProject := filepath.Base(newCwd)
	if newProject == "" || newProject == "." {
		newProject = "Unknown Project"
	}

	s.registry.UpdateActivity(payload.SessionID, session.ActivityRequest{
		SessionID: payload.SessionID,
		Details:   fmt.Sprintf("Working on %s", newProject),
	})

	slog.Info("cwd-changed received",
		"session", payload.SessionID,
		"from", payload.OldCwd,
		"to", newCwd)
	s.hookStats.record("cwd-changed")
}

// SubagentStopPayload is the JSON body from Claude Code SubagentStop hook events.
// Per D-01: carries agent lifecycle data for subagent completion tracking.
type SubagentStopPayload struct {
	SessionID           string `json:"session_id"`
	Cwd                 string `json:"cwd"`
	Description         string `json:"description"`
	AgentType           string `json:"agent_type"`
	AgentID             string `json:"agent_id"`
	AgentTranscriptPath string `json:"agent_transcript_path"`
	TranscriptPath      string `json:"transcript_path"`
}

// handleSubagentStop processes POST /hooks/subagent-stop requests from Claude Code.
// Records subagent completion via the analytics tracker using 4-strategy matching (D-08).
func (s *Server) handleSubagentStop(w http.ResponseWriter, r *http.Request) {
	var payload SubagentStopPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("subagent-stop: invalid JSON body", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Session ID fallback per D-31 pattern
	if payload.SessionID == "" {
		projectName := filepath.Base(payload.Cwd)
		if projectName == "" || projectName == "." {
			projectName = "unknown"
		}
		payload.SessionID = "http-" + projectName
	}

	if s.tracker != nil {
		s.tracker.RecordSubagentComplete(payload.SessionID, analytics.SubagentStopEvent{
			AgentID:     payload.AgentID,
			Description: payload.Description,
			AgentType:   payload.AgentType,
		})
	}

	// Store transcript_path per D-18
	if payload.TranscriptPath != "" {
		s.registry.UpdateTranscriptPath(payload.SessionID, payload.TranscriptPath)
	}

	s.hookStats.record("SubagentStop")
	slog.Debug("subagent-stop received", "session", payload.SessionID, "agentType", payload.AgentType)
	w.WriteHeader(http.StatusOK)
}

// truncate returns the first n characters of s, or s itself if shorter.
// Used for Agent name inference from prompt text.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// handleStatusline processes POST /statusline requests with session metadata.
// Updates model, branch, tokens, and cost data per D-36/D-37.
func (s *Server) handleStatusline(w http.ResponseWriter, r *http.Request) {
	var payload StatuslinePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("statusline: invalid JSON body", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.SessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Get git branch from cwd
	branch := getGitBranch(payload.Cwd)

	s.registry.UpdateSessionData(
		payload.SessionID,
		payload.Model.Name,
		branch,
		payload.TotalTokens,
		payload.TotalCostUSD,
	)

	slog.Debug("statusline updated", "session", payload.SessionID, "model", payload.Model.Name)

	w.WriteHeader(http.StatusOK)
}

// writeJSON writes a JSON response with the given status and logs encode
// failures at Warn level (typically client disconnect after WriteHeader).
func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Warn("failed to encode response", "error", err)
	}
}

// handleConfigUpdate processes POST /config requests for runtime preset/lang changes per D-50.
func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var payload ConfigUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("config: invalid JSON body", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Lang != "" && payload.Lang != "en" && payload.Lang != "de" {
		http.Error(w, `invalid lang: must be "en" or "de"`, http.StatusBadRequest)
		return
	}

	if (payload.Preset != "" || payload.Lang != "") && s.onConfig != nil {
		s.onConfig(payload)
		slog.Info("config updated", "preset", payload.Preset, "lang", payload.Lang)
	}

	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handlePostPreview processes POST /preview requests for temporary presence changes.
// Duration is clamped between 5-300 seconds, defaulting to 60.
// A timer restores normal presence after the duration expires.
// Posting a new preview cancels any existing preview timer.
func (s *Server) handlePostPreview(w http.ResponseWriter, r *http.Request) {
	var payload PreviewPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	duration := payload.Duration
	if duration <= 0 {
		duration = 60
	}
	if duration < 5 {
		duration = 5
	}
	if duration > 300 {
		duration = 300
	}

	// D-11: No "Preview Mode" default text. Empty details/state means the
	// resolver will use preset messages, producing more realistic screenshots.

	s.previewMu.Lock()
	// Cancel existing preview timer if active
	if s.previewState != nil && s.previewState.Timer != nil {
		s.previewState.Timer.Stop()
	}

	s.previewState = &PreviewState{
		Active:    true,
		ExpiresAt: time.Now().Add(time.Duration(duration) * time.Second),
	}

	s.previewState.Timer = time.AfterFunc(time.Duration(duration)*time.Second, func() {
		s.previewMu.Lock()
		s.previewState = nil
		s.previewMu.Unlock()
		if s.onPreviewEnd != nil {
			s.onPreviewEnd()
		}
	})
	s.previewMu.Unlock()

	if s.onPreview != nil {
		s.onPreview(payload, time.Duration(duration)*time.Second)
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"expiresIn": duration,
	})
}

// handleHealth processes GET /health requests with server status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"status":   "ok",
		"sessions": s.registry.SessionCount(),
		"uptime":   time.Since(s.startTime).String(),
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// presetsResponse is the JSON response for GET /presets.
type presetsResponse struct {
	Presets       []presetInfo `json:"presets"`
	ActivePreset  string       `json:"activePreset"`
	DisplayDetail string       `json:"displayDetail"`
}

// presetInfo summarizes a single preset for the GET /presets response.
type presetInfo struct {
	Label         string   `json:"label"`
	Description   string   `json:"description"`
	SampleDetails []string `json:"sampleDetails"`
	SampleState   []string `json:"sampleState"`
	HasButtons    bool     `json:"hasButtons"`
}

// handleGetPresets processes GET /presets requests, returning all available presets
// with sample messages, active preset name, and display detail level.
func (s *Server) handleGetPresets(w http.ResponseWriter, r *http.Request) {
	names := preset.AvailablePresets()

	var infos []presetInfo
	for _, name := range names {
		p, err := preset.LoadPreset(name)
		if err != nil {
			slog.Warn("presets: failed to load preset", "name", name, "error", err)
			continue
		}
		info := presetInfo{
			Label:       name,
			Description: p.Description,
			HasButtons:  len(p.Buttons) > 0,
		}
		// Sample up to 3 details from "coding" category
		if pool, ok := p.SingleSessionDetails["coding"]; ok && len(pool) > 0 {
			end := 3
			if len(pool) < end {
				end = len(pool)
			}
			info.SampleDetails = pool[:end]
		}
		// Sample up to 3 state messages
		if len(p.SingleSessionState) > 0 {
			end := 3
			if len(p.SingleSessionState) < end {
				end = len(p.SingleSessionState)
			}
			info.SampleState = p.SingleSessionState[:end]
		}
		infos = append(infos, info)
	}

	// Sort presets by label for deterministic output
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Label < infos[j].Label
	})

	cfg := ServerConfig{Preset: "minimal", DisplayDetail: "minimal"}
	if s.configGetter != nil {
		cfg = s.configGetter()
	}

	resp := presetsResponse{
		Presets:       infos,
		ActivePreset:  cfg.Preset,
		DisplayDetail: cfg.DisplayDetail,
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// statusAnalytics holds aggregated analytics across all sessions for the
// GET /status response per D-24.
type statusAnalytics struct {
	TotalTokens    analytics.TokenBreakdown           `json:"totalTokens"`
	TotalCost      float64                            `json:"totalCost"`
	TotalTools     map[string]int                     `json:"totalTools"`
	Compactions    int                                `json:"compactions"`
	SessionDetails map[string]*analytics.TrackerState `json:"sessions"`
}

// statusResponse is the JSON response for GET /status.
type statusResponse struct {
	Version          string             `json:"version"`
	Uptime           string             `json:"uptime"`
	DiscordConnected bool               `json:"discordConnected"`
	Config           ServerConfig       `json:"config"`
	Sessions         []*session.Session `json:"sessions"`
	HookStats        hookStatsJSON      `json:"hookStats"`
	Analytics        *statusAnalytics   `json:"analytics,omitempty"`
}

// handleGetStatus processes GET /status requests, returning comprehensive
// server status including version, uptime, config, sessions, and hook stats.
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	connected := false
	if s.discordConnected != nil {
		connected = s.discordConnected()
	}

	cfg := ServerConfig{Preset: "minimal", DisplayDetail: "minimal"}
	if s.configGetter != nil {
		cfg = s.configGetter()
	}

	sessions := s.registry.GetAllSessions()

	resp := statusResponse{
		Version:          s.version,
		Uptime:           time.Since(s.startTime).Truncate(time.Second).String(),
		DiscordConnected: connected,
		Config:           cfg,
		Sessions:         sessions,
		HookStats:        s.hookStats.toJSON(),
	}

	// Aggregate analytics across all sessions per D-24
	if s.tracker != nil {
		sa := &statusAnalytics{
			TotalTools:     make(map[string]int),
			SessionDetails: make(map[string]*analytics.TrackerState),
		}

		for _, sess := range sessions {
			state := s.tracker.GetState(sess.SessionID)
			stateCopy := state
			sa.SessionDetails[sess.SessionID] = &stateCopy

			// Aggregate tokens
			for _, tb := range state.Tokens {
				sa.TotalTokens.Input += tb.Input
				sa.TotalTokens.Output += tb.Output
				sa.TotalTokens.CacheRead += tb.CacheRead
				sa.TotalTokens.CacheWrite += tb.CacheWrite
			}

			// Aggregate tools
			for tool, count := range state.ToolCounts {
				sa.TotalTools[tool] += count
			}

			// Aggregate compactions
			sa.Compactions += state.CompactionCount

			// Aggregate cost (cache-aware per D-22)
			sa.TotalCost += analytics.CalculateSessionCost(state.Tokens, state.Baselines)
		}

		resp.Analytics = sa
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleSessions processes GET /sessions requests returning all active sessions.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.registry.GetAllSessions()

	s.writeJSON(w, http.StatusOK, sessions)
}

// previewMessage represents a single resolved Activity message for preview.
type previewMessage struct {
	Details    string `json:"details"`
	State      string `json:"state"`
	SmallImage string `json:"smallImage"`
	SmallText  string `json:"smallText"`
	LargeText  string `json:"largeText"`
	TimeWindow string `json:"timeWindow"` // Human-readable window label
}

// handleGetPreviewMessages returns resolved Activity messages from a preset
// for message rotation preview. Shows how StablePick rotates messages across
// different time windows per D-08 and D-10.
//
// Query params:
//
//	preset   - preset name (default: "minimal")
//	activity - SmallImageKey (default: "coding")
//	count    - number of time windows to simulate (default: 5, max: 20) [T-16-04-01]
//	detail   - displayDetail level (default: "standard")
func (s *Server) handleGetPreviewMessages(w http.ResponseWriter, r *http.Request) {
	presetName := r.URL.Query().Get("preset")
	if presetName == "" {
		presetName = "minimal"
	}

	activity := r.URL.Query().Get("activity")
	if activity == "" {
		activity = "coding"
	}

	countStr := r.URL.Query().Get("count")
	count := 5
	if countStr != "" {
		if parsed, err := strconv.Atoi(countStr); err == nil && parsed > 0 {
			if parsed > 20 {
				parsed = 20
			}
			count = parsed
		}
	}

	detailStr := r.URL.Query().Get("detail")
	if detailStr == "" {
		detailStr = "standard"
	}
	detail := config.ParseDisplayDetail(detailStr)

	// Load the requested preset (T-16-04-03: invalid name returns 400)
	p, err := preset.LoadPreset(presetName)
	if err != nil {
		http.Error(w, fmt.Sprintf("preset %q not found", presetName), http.StatusBadRequest)
		return
	}

	// Build a realistic fake session for placeholder resolution per D-10
	fakeSession := &session.Session{
		SessionID:      "preview-demo",
		ProjectPath:    "/home/user/my-saas-app",
		ProjectName:    "my-saas-app",
		PID:            12345,
		Model:          "Opus 4.6",
		Branch:         "feature/auth",
		Details:        "Working on my-saas-app",
		SmallImageKey:  activity,
		SmallText:      "Editing files",
		TotalTokens:    250000,
		TotalCostUSD:   2.50,
		Status:         session.StatusActive,
		StartedAt:      time.Now().Add(-2 * time.Hour),
		LastActivityAt: time.Now(),
		LastFile:       "auth.ts",
		LastFilePath:   "src/lib/auth.ts",
		LastCommand:    "npm test",
		LastQuery:      "authentication",
	}

	// Simulate StablePick across time windows (5-minute intervals)
	seed := resolver.HashString(fakeSession.SessionID)
	baseTime := time.Now()

	messages := make([]previewMessage, 0, count)
	for i := 0; i < count; i++ {
		windowTime := baseTime.Add(time.Duration(i*5) * time.Minute)

		// Get message pool for this activity
		pool := p.SingleSessionDetails[activity]
		if len(pool) == 0 {
			pool = p.SingleSessionDetailsFallback
		}

		details := resolver.StablePick(pool, seed, windowTime)
		duration := windowTime.Sub(fakeSession.StartedAt)
		values := buildPreviewPlaceholderValues(fakeSession, detail, duration)
		details = replacePlaceholdersSimple(details, values)

		statePool := p.SingleSessionState
		state := resolver.StablePick(statePool, seed+1, windowTime)
		state = replacePlaceholdersSimple(state, values)

		messages = append(messages, previewMessage{
			Details:    details,
			State:      state,
			SmallImage: activity,
			SmallText:  fakeSession.SmallText,
			LargeText:  fakeSession.ProjectName + " (" + fakeSession.Branch + ")",
			TimeWindow: fmt.Sprintf("Window %d (%d-%d min)", i+1, i*5, (i+1)*5),
		})
	}

	s.writeJSON(w, http.StatusOK, messages)
}

// buildPreviewPlaceholderValues creates placeholder values for preview messages.
// Mirrors resolver placeholder logic without requiring internal resolver access.
func buildPreviewPlaceholderValues(s *session.Session, detail config.DisplayDetail, duration time.Duration) map[string]string {
	values := map[string]string{
		"{activity}": s.SmallText,
		"{sessions}": "1",
		"{duration}": formatPreviewDuration(duration),
		"{filepath}": s.LastFilePath,
	}
	switch detail {
	case config.DetailMinimal:
		values["{project}"] = s.ProjectName
	case config.DetailStandard, config.DetailVerbose:
		values["{project}"] = s.ProjectName
		values["{branch}"] = s.Branch
		values["{file}"] = s.LastFile
		values["{command}"] = s.LastCommand
		values["{query}"] = s.LastQuery
		values["{model}"] = s.Model
		values["{tokens}"] = fmt.Sprintf("%.0fK", float64(s.TotalTokens)/1000)
		values["{cost}"] = fmt.Sprintf("$%.2f", s.TotalCostUSD)
	case config.DetailPrivate:
		values["{project}"] = "Project"
	}
	return values
}

// formatPreviewDuration formats a duration as "Xh Ym" for preview.
func formatPreviewDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// replacePlaceholdersSimple replaces {key} placeholders in a string.
func replacePlaceholdersSimple(s string, values map[string]string) string {
	for k, v := range values {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

// Start starts the HTTP server and blocks until the context is cancelled.
// addr defaults to "127.0.0.1" per D-55. Performs graceful shutdown with a 5s timeout.
func (s *Server) Start(ctx context.Context, addr string, port int) error {
	if addr == "" {
		addr = "127.0.0.1"
	}

	s.httpServer = &http.Server{
		Addr:    net.JoinHostPort(addr, strconv.Itoa(port)),
		Handler: s.Handler(),
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP server starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server failed to start: %w", err)
	case <-ctx.Done():
		slog.Info("HTTP server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}
		return nil
	}
}

// getGitBranch returns the current git branch for a directory path.
// Returns an empty string if the directory is not a git repository or on error.
func getGitBranch(dir string) string {
	if dir == "" {
		return ""
	}

	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	branch := strings.TrimSpace(string(output))

	// If HEAD (detached), try symbolic-ref
	if branch == "HEAD" {
		cmd = exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD")
		output, err = cmd.Output()
		if err == nil {
			branch = strings.TrimSpace(string(output))
		}
	}

	return branch
}

// sanitizeWindowsJSON fixes unescaped backslashes in JSON strings.
// Claude Code on Windows may send paths like "C:\Users\..." instead of
// "C:\\Users\\..." (known bug anthropics/claude-code#20015).
// This replaces \X with \\X when X is not a valid JSON escape character.
func sanitizeWindowsJSON(data []byte) []byte {
	// Valid JSON escape characters after backslash: " \ / b f n r t u
	result := make([]byte, 0, len(data)+32)
	inString := false
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if ch == '"' && (i == 0 || data[i-1] != '\\') {
			inString = !inString
		}
		if inString && ch == '\\' && i+1 < len(data) {
			next := data[i+1]
			switch next {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
				// Valid JSON escape — keep as-is
			default:
				// Invalid escape like \U, \k, \P — add extra backslash
				result = append(result, '\\')
			}
		}
		result = append(result, ch)
	}
	return result
}

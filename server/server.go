// Package server provides the HTTP server that receives Claude Code hook events
// and statusline data, translating them into session activity updates.
package server

import (
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/tsanva/cc-discord-presence/preset"
	"github.com/tsanva/cc-discord-presence/session"
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
	StartTimestamp int64         `json:"startTimestamp,omitempty"`  // Unix epoch, per D-07
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

	mux.HandleFunc("POST /hooks/{hookType}", s.handleHook)
	mux.HandleFunc("POST /statusline", s.handleStatusline)
	mux.HandleFunc("POST /config", s.handleConfigUpdate)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /sessions", s.handleSessions)
	mux.HandleFunc("GET /presets", s.handleGetPresets)
	mux.HandleFunc("GET /status", s.handleGetStatus)
	mux.HandleFunc("POST /preview", s.handlePostPreview)

	return mux
}

// handleHook processes POST /hooks/{hookType} requests from Claude Code.
// It maps the hook event to an activity and updates the session registry.
func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	hookType := r.PathValue("hookType")

	var payload HookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("hook: invalid JSON body", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
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

	s.hookStats.record(hookType)

	slog.Debug("hook received", "hookType", hookType, "tool", payload.ToolName, "session", payload.SessionID)

	w.WriteHeader(http.StatusOK)
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

// handleConfigUpdate processes POST /config requests for runtime preset changes per D-50.
func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var payload ConfigUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Debug("config: invalid JSON body", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Preset != "" && s.onConfig != nil {
		s.onConfig(payload)
		slog.Info("config updated", "preset", payload.Preset)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// statusResponse is the JSON response for GET /status.
type statusResponse struct {
	Version          string             `json:"version"`
	Uptime           string             `json:"uptime"`
	DiscordConnected bool               `json:"discordConnected"`
	Config           ServerConfig       `json:"config"`
	Sessions         []*session.Session `json:"sessions"`
	HookStats        hookStatsJSON      `json:"hookStats"`
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

	resp := statusResponse{
		Version:          s.version,
		Uptime:           time.Since(s.startTime).Truncate(time.Second).String(),
		DiscordConnected: connected,
		Config:           cfg,
		Sessions:         s.registry.GetAllSessions(),
		HookStats:        s.hookStats.toJSON(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSessions processes GET /sessions requests returning all active sessions.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.registry.GetAllSessions()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(sessions)
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

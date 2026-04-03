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
	"strconv"
	"strings"
	"time"

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
type HookPayload struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	ToolName  string `json:"tool_name,omitempty"`
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

// Server manages the HTTP server for hook events.
type Server struct {
	registry   *session.SessionRegistry
	onConfig   func(ConfigUpdatePayload)
	httpServer *http.Server
	startTime  time.Time
}

// NewServer creates a new Server.
// registry: session registry for activity updates
// onConfig: callback for config change requests (may be nil)
func NewServer(registry *session.SessionRegistry, onConfig func(ConfigUpdatePayload)) *Server {
	return &Server{
		registry:  registry,
		onConfig:  onConfig,
		startTime: time.Now(),
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

	if payload.SessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	icon, text := MapHookToActivity(hookType, payload.ToolName)

	// Derive Details from the project name
	details := filepath.Base(payload.Cwd)
	if details == "" || details == "." {
		details = "Unknown Project"
	}

	req := session.ActivityRequest{
		SessionID:     payload.SessionID,
		Cwd:           payload.Cwd,
		ToolName:      payload.ToolName,
		SmallImageKey: icon,
		SmallText:     text,
		Details:       fmt.Sprintf("Working on %s", details),
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

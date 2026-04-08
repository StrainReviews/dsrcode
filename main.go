package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/tsanva/cc-discord-presence/analytics"
	"github.com/tsanva/cc-discord-presence/config"
	"github.com/tsanva/cc-discord-presence/discord"
	"github.com/tsanva/cc-discord-presence/logger"
	"github.com/tsanva/cc-discord-presence/preset"
	"github.com/tsanva/cc-discord-presence/resolver"
	"github.com/tsanva/cc-discord-presence/server"
	"github.com/tsanva/cc-discord-presence/session"
)

// Version of the daemon (overridable via -ldflags "-X main.Version=x.y.z")
var Version = "3.2.0"

const (
	// Discord Application ID for "DSR Code"
	ClientID = "1489600745295708160"

	// Polling interval for JSONL fallback watcher
	PollInterval = 3 * time.Second

	// discordRateLimit is the minimum interval between Discord SetActivity calls.
	// Discord rate-limits activity updates to once every 15 seconds.
	discordRateLimit = 15 * time.Second

	// debounceDelay is the time to wait after the last registry change before
	// resolving a new presence. Collapses rapid-fire tool events.
	debounceDelay = 100 * time.Millisecond
)

// Model display names - add new model IDs here when released
var modelDisplayNames = map[string]string{
	"claude-opus-4-6":             "Opus 4.6",
	"claude-sonnet-4-6":           "Sonnet 4.6",
	"claude-haiku-4-5":            "Haiku 4.5",
	"claude-opus-4-5-20251101":    "Opus 4.5",
	"claude-sonnet-4-5-20241022":  "Sonnet 4.5",
	"claude-sonnet-4-20250514":    "Sonnet 4",
	"claude-haiku-4-5-20241022":   "Haiku 4.5",
}

// StatusLineData matches Claude Code's statusline JSON structure
type StatusLineData struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Model     struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
	Cost struct {
		TotalCostUSD       float64 `json:"total_cost_usd"`
		TotalDurationMS    int64   `json:"total_duration_ms"`
		TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
	} `json:"cost"`
	ContextWindow struct {
		TotalInputTokens  int64    `json:"total_input_tokens"`
		TotalOutputTokens int64    `json:"total_output_tokens"`
		UsedPercentage    *float64 `json:"used_percentage"`
	} `json:"context_window"`
}

// SessionData holds parsed session information (used by JSONL fallback)
type SessionData struct {
	ProjectName     string
	ProjectPath     string
	GitBranch       string
	ModelName       string
	TotalTokens     int64
	TotalCost       float64
	StartTime       time.Time
	CompactionCount int
}

// JSONLMessage represents a message entry in JSONL files.
// Extended for Phase 17 with cache token fields and compaction detection.
type JSONLMessage struct {
	Type             string `json:"type"`
	Timestamp        string `json:"timestamp"`
	Cwd              string `json:"cwd"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	UUID             string `json:"uuid"`
	Message          struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens             int64 `json:"input_tokens"`
			OutputTokens            int64 `json:"output_tokens"`
			CacheReadInputTokens    int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// CLI flags
var (
	flagPort    = flag.Int("port", 0, "HTTP server port (default 19460)")
	flagPreset  = flag.String("preset", "", "Display preset name")
	flagVerbose = flag.Bool("v", false, "Debug logging")
	flagQuiet   = flag.Bool("q", false, "Error-only logging")
	flagConfig  = flag.String("config", "", "Config file path")
	flagVersion = flag.Bool("version", false, "Print version and exit")
)

// Package-level state used by JSONL fallback and tests
var (
	claudeDir             string
	projectsDir           string
	dataFilePath          string
	sessionStartTime      = time.Now()
	jsonlDeprecationOnce  sync.Once
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}
	claudeDir = filepath.Join(home, ".claude")
	projectsDir = filepath.Join(claudeDir, "projects")
	dataFilePath = filepath.Join(claudeDir, "discord-presence-data.json")
}

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Println("cc-discord-presence " + Version)
		os.Exit(0)
	}

	// 1. Load config (CLI flags > env > file > defaults)
	cfg := config.LoadConfig(*flagPort, *flagPreset, *flagVerbose, *flagQuiet, *flagConfig)

	// 2. Setup structured logger
	logger.Setup(cfg.LogFile, cfg.LogLevel)

	slog.Info("starting cc-discord-presence",
		"version", Version,
		"port", cfg.Port,
		"preset", cfg.Preset,
	)

	// 3. Create analytics tracker per D-09/D-34
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: cfg.Features.Analytics},
		DataDir:  filepath.Join(claudeDir, "discord-presence-analytics"),
	})

	// 4. Load preset with language selection (D-29)
	var presetMu sync.RWMutex
	currentPreset := preset.MustLoadPresetWithLang(cfg.Preset, cfg.Lang)
	slog.Info("preset loaded", "name", cfg.Preset, "lang", cfg.Lang)

	var cfgMu sync.RWMutex

	// 5. Context with cancel for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 5. Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 6. Determine Discord client ID (config > legacy constant)
	clientID := cfg.DiscordClientID
	if clientID == "" {
		clientID = ClientID
	}
	discordClient := discord.NewClient(clientID)

	// 7. Presence update debounce channel
	updateChan := make(chan struct{}, 1)

	// 8. Session registry with onChange wired to debounce channel
	registry := session.NewRegistry(func() {
		select {
		case updateChan <- struct{}{}:
		default: // non-blocking
		}
	})

	// 9. Discord connection state (atomic for thread-safe reads)
	var discordConnected atomic.Bool

	// 10. Discord connection loop (goroutine)
	go discordConnectionLoop(ctx, discordClient, &discordConnected)

	// 11. Presence update debouncer (goroutine)
	go presenceDebouncer(ctx, updateChan, registry, &presetMu, &currentPreset, discordClient, func() config.DisplayDetail {
		cfgMu.RLock()
		defer cfgMu.RUnlock()
		return cfg.DisplayDetail
	})

	// 12. HTTP server
	srv := server.NewServer(
		registry,
		func(payload server.ConfigUpdatePayload) {
			if payload.Preset == "" && payload.Lang == "" {
				return
			}

			cfgMu.RLock()
			presetName := cfg.Preset
			lang := cfg.Lang
			cfgMu.RUnlock()

			if payload.Preset != "" {
				presetName = payload.Preset
			}
			if payload.Lang != "" {
				lang = payload.Lang
			}

			p, err := preset.LoadPresetWithLang(presetName, lang)
			if err != nil {
				slog.Warn("config update: invalid preset/lang", "preset", presetName, "lang", lang, "error", err)
				return
			}

			presetMu.Lock()
			currentPreset = p
			presetMu.Unlock()

			cfgMu.Lock()
			cfg.Preset = presetName
			cfg.Lang = lang
			cfgMu.Unlock()

			slog.Info("preset reloaded via /config", "preset", presetName, "lang", lang)
		},
		Version,
		func() server.ServerConfig {
			cfgMu.RLock()
			defer cfgMu.RUnlock()
			return server.ServerConfig{
				Preset:        cfg.Preset,
				DisplayDetail: string(cfg.DisplayDetail),
				Port:          cfg.Port,
				BindAddr:      cfg.BindAddr,
				Lang:          cfg.Lang,
			}
		},
		func() bool {
			return discordConnected.Load()
		},
		func(payload server.PreviewPayload, duration time.Duration) {
			activity := discord.Activity{
				Details:    payload.Details,
				State:      payload.State,
				LargeImage: "dsr-code",
				LargeText:  payload.LargeText,
			}
			// Apply extended fields per D-07
			if payload.SmallImage != "" {
				activity.SmallImage = payload.SmallImage
			} else {
				activity.SmallImage = "thinking"
			}
			if payload.SmallText != "" {
				activity.SmallText = payload.SmallText
			}
			if payload.StartTimestamp > 0 {
				ts := time.Unix(payload.StartTimestamp, 0)
				activity.StartTime = &ts
			}
			if err := discordClient.SetActivity(activity); err != nil {
				slog.Debug("preview SetActivity failed", "error", err)
			}
		},
		func() {
			// Trigger normal presence resolution by signaling the debounce channel
			select {
			case updateChan <- struct{}{}:
			default:
			}
		},
	)
	srv.SetTracker(tracker)

	go func() {
		if err := srv.Start(ctx, cfg.BindAddr, cfg.Port); err != nil {
			slog.Error("HTTP server error", "error", err)
			cancel() // Fatal: shut down daemon if HTTP server can't bind
		}
	}()

	// 13. Stale session checker
	go session.CheckStaleSessions(ctx, registry, cfg.IdleTimeout, cfg.RemoveTimeout, cfg.StaleCheckInterval)

	// 14. Config file watcher
	configPath := *flagConfig
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}
	if err := config.WatchConfig(ctx, configPath, func(newCfg config.Config) {
		slog.Info("config file reloaded")
		// Update log level
		logger.SetLevel(newCfg.LogLevel)
		// Reload preset if changed
		// Reload preset if preset or lang changed
		if newCfg.Preset != cfg.Preset || newCfg.Lang != cfg.Lang {
			lang := newCfg.Lang
			if lang == "" {
				lang = cfg.Lang
			}
			p, err := preset.LoadPresetWithLang(newCfg.Preset, lang)
			if err != nil {
				slog.Warn("config watcher: invalid preset", "preset", newCfg.Preset, "error", err)
				return
			}
			presetMu.Lock()
			currentPreset = p
			presetMu.Unlock()
			slog.Info("preset reloaded via config watcher", "preset", newCfg.Preset, "lang", lang)
		}
		// Update config fields under lock for hot-reload
		cfgMu.Lock()
		cfg.Preset = newCfg.Preset
		cfg.DisplayDetail = newCfg.DisplayDetail
		cfg.Lang = newCfg.Lang
		cfg.Features = newCfg.Features
		cfgMu.Unlock()
	}); err != nil {
		slog.Warn("config watcher failed to start", "error", err)
	}

	// 15. JSONL fallback watcher (per D-51: zero-config install-and-forget)
	go jsonlFallbackWatcher(ctx, registry, tracker)

	slog.Info("all subsystems started, waiting for shutdown signal")

	// 16. Wait for shutdown signal
	select {
	case <-sigChan:
		slog.Info("shutdown signal received")
	case <-ctx.Done():
	}

	cancel()
	discordClient.Close()
	slog.Info("shutdown complete")
}

// discordConnectionLoop connects to Discord IPC with exponential backoff.
// Retries until ctx is cancelled. Per D-42 and D-43.
// Sets connected to true on successful IPC handshake, false on shutdown.
func discordConnectionLoop(ctx context.Context, client *discord.Client, connected *atomic.Bool) {
	backoff := discord.DefaultBackoff()

	for {
		select {
		case <-ctx.Done():
			connected.Store(false)
			return
		default:
		}

		if err := client.Connect(); err != nil {
			connected.Store(false)
			delay := backoff.Next()
			slog.Debug("discord connection failed, retrying", "error", err, "retry_in", delay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
				continue
			}
		}

		slog.Info("discord IPC connected")
		connected.Store(true)
		backoff.Reset()

		// Stay connected until context cancelled. The discord client
		// does not provide a blocking call, so we wait for shutdown.
		<-ctx.Done()
		connected.Store(false)
		return
	}
}

// presenceDebouncer listens for registry change signals and updates Discord
// presence with rate limiting (15s) and debouncing (100ms).
// displayDetailGetter returns the current DisplayDetail level from config.
func presenceDebouncer(
	ctx context.Context,
	updateChan <-chan struct{},
	registry *session.SessionRegistry,
	presetMu *sync.RWMutex,
	currentPreset **preset.MessagePreset,
	discordClient *discord.Client,
	displayDetailGetter func() config.DisplayDetail,
) {
	var lastUpdate time.Time
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		case <-updateChan:
			// Debounce: wait 100ms after last signal
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				// Rate limit: skip if < 15s since last SetActivity
				if time.Since(lastUpdate) < discordRateLimit {
					slog.Debug("presence update skipped (rate limit)")
					return
				}

				sessions := registry.GetAllSessions()
				presetMu.RLock()
				p := *currentPreset
				presetMu.RUnlock()

				detail := displayDetailGetter()
				activity := resolver.ResolvePresence(sessions, p, detail, time.Now())

				if activity != nil {
					if err := discordClient.SetActivity(*activity); err != nil {
						slog.Debug("discord SetActivity failed", "error", err)
						return
					}
				}

				lastUpdate = time.Now()
				slog.Debug("presence updated", "sessions", len(sessions))
			})
		}
	}
}

// jsonlFallbackWatcher watches JSONL files for changes and feeds session data
// into the registry as passive sessions. This provides the D-51 "zero-config
// install and forget" experience.
func jsonlFallbackWatcher(ctx context.Context, registry *session.SessionRegistry, tracker *analytics.Tracker) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Debug("JSONL watcher: fsnotify not available, using polling", "error", err)
		jsonlPollLoop(ctx, registry, tracker)
		return
	}
	defer watcher.Close()

	// Watch the main claude dir for statusline data changes
	if err := watcher.Add(claudeDir); err != nil {
		slog.Debug("JSONL watcher: cannot watch claude dir, using polling", "error", err)
		jsonlPollLoop(ctx, registry, tracker)
		return
	}

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if filepath.Base(event.Name) == "discord-presence-data.json" {
				ingestJSONLFallback(registry, tracker)
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		case <-ticker.C:
			ingestJSONLFallback(registry, tracker)
		}
	}
}

// jsonlPollLoop is the fallback polling mode for JSONL data when fsnotify
// is not available.
func jsonlPollLoop(ctx context.Context, registry *session.SessionRegistry, tracker *analytics.Tracker) {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ingestJSONLFallback(registry, tracker)
		}
	}
}

// ingestJSONLFallback reads session data from statusline file or JSONL files
// and feeds it into the registry as a passive session.
func ingestJSONLFallback(registry *session.SessionRegistry, tracker *analytics.Tracker) {
	// Skip JSONL fallback when real Claude sessions exist (any project).
	// The JSONL watcher may pick up a different project's file, creating a
	// phantom session alongside the real one. Real sessions have better data.
	if registry.HasHigherRankSessions(session.SourceJSONL) {
		// Also remove any previously created JSONL sessions — they may have
		// been registered before the real session arrived (race at startup).
		registry.RemoveSessionsBySource(session.SourceJSONL)
		return
	}

	sessionData := readSessionData(tracker)
	if sessionData == nil {
		return
	}

	// Use a synthetic session ID for the JSONL-based session
	syntheticID := "jsonl-" + sessionData.ProjectName

	jsonlDeprecationOnce.Do(func() {
		slog.Warn("JSONL fallback active -- configure HTTP hooks for better accuracy",
			"project", sessionData.ProjectName,
			"hint", "Run /dsrcode:setup to migrate to HTTP hooks",
		)
	})

	// Ensure the passive session exists (registry handles dedup via D-02)
	existing := registry.GetSession(syntheticID)
	if existing == nil {
		registry.StartSession(session.ActivityRequest{
			SessionID: syntheticID,
			Cwd:       sessionData.ProjectPath,
			Details:   fmt.Sprintf("Working on %s", sessionData.ProjectName),
		}, 0)
	}

	// Update session metadata from JSONL data
	registry.UpdateSessionData(
		syntheticID,
		sessionData.ModelName,
		sessionData.GitBranch,
		sessionData.TotalTokens,
		sessionData.TotalCost,
	)

	// Feed JSONL compaction count into tracker so syncAnalyticsToRegistry picks it up
	for i := range sessionData.CompactionCount {
		tracker.RecordCompaction(syntheticID, fmt.Sprintf("jsonl-compact-%d", i))
	}

	// Sync analytics state to registry for resolver access
	syncAnalyticsToRegistry(tracker, registry, syntheticID)
}

// ---------------------------------------------------------------------------
// JSONL fallback functions (preserved for backward compatibility and tests)
// ---------------------------------------------------------------------------

func readStatusLineData(tracker *analytics.Tracker) *SessionData {
	data, err := os.ReadFile(dataFilePath)
	if err != nil {
		return nil
	}

	var statusLine StatusLineData
	if err := json.Unmarshal(data, &statusLine); err != nil {
		return nil
	}

	if statusLine.SessionID == "" {
		return nil
	}

	projectPath := statusLine.Workspace.ProjectDir
	if projectPath == "" {
		projectPath = statusLine.Cwd
	}

	projectName := filepath.Base(projectPath)
	if projectName == "" || projectName == "." {
		projectName = "Unknown Project"
	}

	// Extract context% and feed to analytics tracker per D-10
	if tracker != nil && statusLine.ContextWindow.UsedPercentage != nil {
		tracker.UpdateContextUsage(statusLine.SessionID, *statusLine.ContextWindow.UsedPercentage)
	}

	return &SessionData{
		ProjectName: projectName,
		ProjectPath: projectPath,
		GitBranch:   getGitBranch(projectPath),
		ModelName:   statusLine.Model.DisplayName,
		TotalTokens: statusLine.ContextWindow.TotalInputTokens + statusLine.ContextWindow.TotalOutputTokens,
		TotalCost:   statusLine.Cost.TotalCostUSD,
		StartTime:   sessionStartTime,
	}
}

func getGitBranch(projectPath string) string {
	if projectPath == "" {
		return ""
	}

	cmd := exec.Command("git", "-C", projectPath, "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	branch := strings.TrimSpace(string(output))

	// If HEAD (no commits yet), try to get the branch name from symbolic-ref
	if branch == "HEAD" {
		cmd = exec.Command("git", "-C", projectPath, "symbolic-ref", "--short", "HEAD")
		output, err = cmd.Output()
		if err == nil {
			branch = strings.TrimSpace(string(output))
		}
	}

	return branch
}

// findMostRecentJSONL finds the most recently modified JSONL file in ~/.claude/projects/
func findMostRecentJSONL() (string, string, error) {
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return "", "", fmt.Errorf("projects directory does not exist")
	}

	type jsonlFile struct {
		path        string
		projectPath string
		modTime     time.Time
	}

	var files []jsonlFile

	err := filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Extract project path from the directory structure
		// ~/.claude/projects/<encoded-path>/<session>.jsonl
		relPath, _ := filepath.Rel(projectsDir, path)
		parts := strings.SplitN(relPath, string(filepath.Separator), 2)
		if len(parts) < 1 {
			return nil
		}

		// Decode the project path
		encodedPath := parts[0]
		projectPath := strings.ReplaceAll(encodedPath, "--", "\x00")
		projectPath = strings.ReplaceAll(projectPath, "-", "/")
		projectPath = strings.ReplaceAll(projectPath, "\x00", "-")

		files = append(files, jsonlFile{
			path:        path,
			projectPath: projectPath,
			modTime:     info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return "", "", err
	}

	if len(files) == 0 {
		return "", "", fmt.Errorf("no JSONL files found")
	}

	// Sort by modification time, most recent first
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	return files[0].path, files[0].projectPath, nil
}

// parseJSONLSession parses a JSONL file and extracts session data.
// Extended for Phase 17: per-model token breakdown, compaction detection,
// and cache-aware cost calculation via analytics package.
func parseJSONLSession(jsonlPath, _ string) *SessionData {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var (
		totalInputTokens  int64
		totalOutputTokens int64
		lastModel         string
		projectPath       string
		compactionCount   int
		perModelTokens    = make(map[string]analytics.TokenBreakdown)
	)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var msg JSONLMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Cwd != "" && projectPath == "" {
			projectPath = msg.Cwd
		}

		if msg.IsCompactSummary {
			compactionCount++
		}

		if msg.Type == "assistant" && msg.Message.Model != "" {
			lastModel = msg.Message.Model
			totalInputTokens += msg.Message.Usage.InputTokens
			totalOutputTokens += msg.Message.Usage.OutputTokens

			// Accumulate per-model token breakdown for cache-aware pricing
			existing := perModelTokens[msg.Message.Model]
			perModelTokens[msg.Message.Model] = analytics.TokenBreakdown{
				Input:      existing.Input + msg.Message.Usage.InputTokens,
				Output:     existing.Output + msg.Message.Usage.OutputTokens,
				CacheRead:  existing.CacheRead + msg.Message.Usage.CacheReadInputTokens,
				CacheWrite: existing.CacheWrite + msg.Message.Usage.CacheCreationInputTokens,
			}
		}
	}

	if lastModel == "" {
		return nil
	}

	// Use cache-aware cost calculation per D-16/D-17
	totalCost := analytics.CalculateSessionCost(perModelTokens, nil)
	modelName := formatModelName(lastModel)

	projectName := filepath.Base(projectPath)
	if projectName == "" || projectName == "." {
		projectName = "Unknown Project"
	}

	return &SessionData{
		ProjectName:     projectName,
		ProjectPath:     projectPath,
		GitBranch:       getGitBranch(projectPath),
		ModelName:       modelName,
		TotalTokens:     totalInputTokens + totalOutputTokens,
		TotalCost:       totalCost,
		StartTime:       sessionStartTime,
		CompactionCount: compactionCount,
	}
}

// formatModelName converts model ID to display name
func formatModelName(modelID string) string {
	if name, ok := modelDisplayNames[modelID]; ok {
		return name
	}

	if strings.Contains(modelID, "opus") {
		return "Opus"
	}
	if strings.Contains(modelID, "sonnet") {
		return "Sonnet"
	}
	if strings.Contains(modelID, "haiku") {
		return "Haiku"
	}

	return "Claude"
}

// readSessionData tries statusline data first, then falls back to JSONL parsing
func readSessionData(tracker *analytics.Tracker) *SessionData {
	if data := readStatusLineData(tracker); data != nil {
		return data
	}

	jsonlPath, projectPath, err := findMostRecentJSONL()
	if err != nil {
		return nil
	}

	return parseJSONLSession(jsonlPath, projectPath)
}

// syncAnalyticsToRegistry reads the current analytics state from the tracker
// and writes it into the session registry for resolver access. Uses
// json.Marshal to bridge the analytics types into json.RawMessage fields
// on the Session struct (avoiding circular deps per Phase 17 design).
func syncAnalyticsToRegistry(tracker *analytics.Tracker, registry *session.SessionRegistry, sessionID string) {
	state := tracker.GetState(sessionID)

	var update session.AnalyticsUpdate

	if len(state.Tokens) > 0 {
		if data, err := json.Marshal(state.Tokens); err == nil {
			update.TokenBreakdownRaw = data
		}
	}
	if len(state.Baselines) > 0 {
		if data, err := json.Marshal(state.Baselines); err == nil {
			update.TokenBaselinesRaw = data
		}
	}
	if len(state.ToolCounts) > 0 {
		update.ToolCounts = state.ToolCounts
	}
	if len(state.Subagents) > 0 {
		if data, err := json.Marshal(state.Subagents); err == nil {
			update.SubagentTreeRaw = data
		}
	}
	update.CompactionCount = state.CompactionCount
	update.ContextUsagePct = state.ContextPct

	// Compute cost breakdown for the session
	cb := analytics.CalculateCostBreakdown(state.Tokens, state.Baselines)
	if cb.Total > 0 {
		if data, err := json.Marshal(cb); err == nil {
			update.CostBreakdownRaw = data
		}
	}

	registry.UpdateAnalytics(sessionID, update)
}

func formatNumber(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	} else if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

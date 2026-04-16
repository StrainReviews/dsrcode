package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/StrainReviews/dsrcode/analytics"
	"github.com/StrainReviews/dsrcode/coalescer"
	"github.com/StrainReviews/dsrcode/config"
	"github.com/StrainReviews/dsrcode/discord"
	"github.com/StrainReviews/dsrcode/logger"
	"github.com/StrainReviews/dsrcode/preset"
	"github.com/StrainReviews/dsrcode/server"
	"github.com/StrainReviews/dsrcode/session"
)

// version, commit, and date are set by GoReleaser default ldflags.
// See: https://goreleaser.com/cookbooks/using-main.version/
var (
	version = "4.1.2"
	commit  = "none"
	date    = "unknown"
)

const (
	// Discord Application ID for "DSR Code"
	ClientID = "1489600745295708160"

	// Rate-limit cadence + debounce live in the coalescer package now —
	// see coalescer.DiscordRateInterval / DiscordRateBurst / DebounceDelay.
	// Phase 8 replaces drop-on-skip with a pending-state buffer + token
	// bucket; no equivalent constants need to exist here.
)

// Model display names - add new model IDs here when released
var modelDisplayNames = map[string]string{
	"claude-opus-4-6":            "Opus 4.6",
	"claude-sonnet-4-6":          "Sonnet 4.6",
	"claude-haiku-4-5":           "Haiku 4.5",
	"claude-opus-4-5-20251101":   "Opus 4.5",
	"claude-sonnet-4-5-20241022": "Sonnet 4.5",
	"claude-sonnet-4-20250514":   "Sonnet 4",
	"claude-haiku-4-5-20241022":  "Haiku 4.5",
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

// claudeDir is the resolved ~/.claude path used as the analytics data root.
// Initialized in init() so main() and analytics.NewTracker can rely on it.
var claudeDir string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}
	claudeDir = filepath.Join(home, ".claude")
}

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Printf("dsrcode %s (%s, %s)\n", version, commit, date)
		os.Exit(0)
	}

	// 1. Load config (CLI flags > env > file > defaults)
	cfg := config.LoadConfig(*flagPort, *flagPreset, *flagVerbose, *flagQuiet, *flagConfig)

	// 2. Setup structured logger
	logger.Setup(cfg.LogFile, cfg.LogLevel)

	slog.Info("starting dsrcode",
		"version", version,
		"port", cfg.Port,
		"preset", cfg.Preset,
	)

	// 3. Create analytics tracker per D-09/D-34
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: cfg.Features.Analytics},
		DataDir:  filepath.Join(claudeDir, "dsrcode-analytics"),
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

	// 7b. Auto-exit signal channel (Plan 06-04, D-04): the registry onChange
	// callback pushes here when sessions mutate, and Server.onAutoExit pushes
	// here when SessionEnd causes refcount=0. The goroutine below reads these
	// pulses, checks SessionCount(), and starts/cancels the grace-period timer.
	autoExitChan := make(chan struct{}, 1)

	// 8. Session registry with onChange wired to BOTH the debounce channel
	// (presence refresh) and the auto-exit channel (D-04 dual trigger).
	registry := session.NewRegistry(func() {
		select {
		case updateChan <- struct{}{}:
		default: // non-blocking
		}
		select {
		case autoExitChan <- struct{}{}:
		default: // non-blocking
		}
	})

	// 9. Discord connection state (atomic for thread-safe reads)
	var discordConnected atomic.Bool

	// 10. Discord connection loop (goroutine)
	go discordConnectionLoop(ctx, discordClient, &discordConnected)

	// 11. Presence update coalescer (goroutine) — Phase 8
	// Replaces the prior drop-on-skip debouncer with a pending-state
	// buffer + token-bucket limiter. Updates arriving inside the cooldown
	// are coalesced and flushed exactly once when the limiter permits —
	// never discarded. getDedupCount is nil here; Plan 08-03 will wire
	// the real dedup-middleware counter.
	presenceCoalescer := coalescer.New(
		updateChan,
		registry,
		&presetMu,
		&currentPreset,
		discordClient,
		func() config.DisplayDetail {
			cfgMu.RLock()
			defer cfgMu.RUnlock()
			return cfg.DisplayDetail
		},
		nil, // getDedupCount — Plan 08-03 supplies
	)
	go presenceCoalescer.Run(ctx)

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
		version,
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

	// Wire Server.onAutoExit to the auto-exit channel (Plan 06-03 hook point,
	// D-04 SessionEnd trigger). Must be called before srv.Start() so no
	// SessionEnd event can land before the callback is installed.
	srv.SetOnAutoExit(func() {
		select {
		case autoExitChan <- struct{}{}:
		default:
		}
	})

	// Wire Server.onAnalyticsSync to syncAnalyticsToRegistry (Plan 06-05).
	// Invoked from PostToolUse, SessionEnd, and PreCompact handlers after
	// the tracker absorbs token updates, this pushes the tracker state
	// (tokens, cost, compactions, tool counts, subagents) into the session
	// registry so the presence resolver can render the latest analytics in
	// Discord Rich Presence. Must be called before srv.Start() so that any
	// hook event landing immediately after Start sees the installed callback
	// (Go memory model: writes before `go` statement happen-before goroutine
	// start, so all HTTP handler goroutines observe the non-nil callback).
	srv.SetOnAnalyticsSync(func(sessionID string) {
		syncAnalyticsToRegistry(tracker, registry, sessionID)
	})

	go func() {
		if err := srv.Start(ctx, cfg.BindAddr, cfg.Port); err != nil {
			slog.Error("HTTP server error", "error", err)
			cancel() // Fatal: shut down daemon if HTTP server can't bind
		}
	}()

	// 13. Stale session checker
	go session.CheckStaleSessions(ctx, registry, cfg.IdleTimeout, cfg.RemoveTimeout, cfg.StaleCheckInterval)

	// 13b. Auto-exit goroutine (Plan 06-04, D-04/D-05/D-06/D-07): monitors
	// the auto-exit channel for refcount=0 signals, starts a grace timer,
	// cancels it on new SessionStart, and triggers graceful shutdown via
	// cancel() when the grace period expires. A configured grace period of
	// 0 disables auto-exit entirely (legacy behavior, daemon runs forever).
	go autoExitLoop(ctx, cancel, registry, autoExitChan, cfg.ShutdownGracePeriod)

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

	slog.Info("all subsystems started, waiting for shutdown signal")

	// 16. Wait for shutdown signal
	select {
	case <-sigChan:
		slog.Info("shutdown signal received")
	case <-ctx.Done():
		slog.Info("shutdown initiated (auto-exit or fatal error)")
	}

	// D-07 shutdown sequence:
	//   1. Clear Discord Rich Presence activity (empty SetActivity over live IPC)
	//   2. Close Discord IPC
	//   3. Cancel context -> triggers server.Server.Start to call
	//      httpServer.Shutdown(5s timeout) for in-flight request drain, stops
	//      stale checker, presence coalescer, config watcher, and auto-exit
	//      goroutine.
	// cancel() is idempotent (context.CancelFunc), so calling it after
	// ctx.Done() already fired is safe.
	// Phase 8 D-23: Halt the Coalescer BEFORE the direct-to-client clear so
	// no new SetActivity call from a still-scheduled AfterFunc can race
	// with the clear frame. Shutdown is idempotent — a later cancel() path
	// hitting Run's defer will see pending already nil and timers already
	// stopped.
	slog.Debug("shutting down presence coalescer")
	presenceCoalescer.Shutdown()

	slog.Debug("clearing Discord activity")
	if err := discordClient.SetActivity(discord.Activity{}); err != nil {
		slog.Debug("clear activity failed", "error", err)
	}
	slog.Debug("closing Discord IPC")
	if err := discordClient.Close(); err != nil {
		slog.Debug("discord close failed", "error", err)
	}
	cancel()
	slog.Info("shutdown complete")
}

// autoExitLoop is the grace-period auto-exit goroutine (D-04/D-05/D-06/D-07).
//
// It consumes pulses from autoExitChan — each pulse is emitted by (1) the
// session.Registry onChange callback (wrapped in main() to also push to this
// channel, catching stale-detector removals) OR (2) the Server.onAutoExit
// hook, which fires after handleSessionEnd's background goroutine removes a
// session and observes SessionCount() == 0. These two sources provide the
// dual trigger required by D-04.
//
// On each pulse the loop re-queries SessionCount():
//   - If it is 0 and no grace timer is running, it starts a time.AfterFunc
//     for gracePeriod. When the timer fires it double-checks the grace
//     context has not been cancelled (atomic race guard) and then calls
//     the shared cancel() to drive main() into the D-07 shutdown sequence.
//   - If it is >0 and a grace timer is running, a new session has arrived
//     during the grace period (D-06 grace-abort). The goroutine cancels the
//     grace context FIRST (so any concurrent timer firing takes the early
//     exit via the grace context check), then stops the timer, then resets
//     the local state. The daemon keeps running.
//
// A zero gracePeriod disables auto-exit entirely (D-05 sentinel). The
// goroutine logs and returns immediately.
//
// On ctx cancellation (shutdown from any path) the deferred cleanup stops
// the timer and cancels the grace context so no orphan callbacks linger.
func autoExitLoop(
	ctx context.Context,
	cancel context.CancelFunc,
	registry *session.SessionRegistry,
	autoExitChan <-chan struct{},
	gracePeriod time.Duration,
) {
	if gracePeriod == 0 {
		slog.Info("auto-exit disabled (shutdownGracePeriod=0)")
		return
	}

	// Grace state — aborting the pending shutdown means calling abortFn
	// (which cancels the captured grace context AND stops the timer). We
	// keep a single abort closure so the lostcancel vet check sees every
	// context cancellation wired to a defer/abort path.
	var abortFn func()

	defer func() {
		if abortFn != nil {
			abortFn()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-autoExitChan:
			count := registry.SessionCount()

			if count == 0 {
				if abortFn != nil {
					// Already counting down — another mutation landed (e.g.
					// idle transition) but the count is still 0. Leave the
					// existing timer running.
					continue
				}
				graceCtx, graceCancel := context.WithCancel(ctx)
				slog.Info("all sessions ended, starting shutdown grace period",
					"duration", gracePeriod)

				timer := time.AfterFunc(gracePeriod, func() {
					// Double-check the grace context before firing cancel().
					// If a new session arrived during the window between the
					// timer firing and this callback being scheduled, the
					// grace context will already be cancelled and we must
					// not terminate the daemon.
					if graceCtx.Err() != nil {
						return
					}
					slog.Info("grace period expired, initiating auto-exit")
					cancel()
				})

				// Compose the abort closure: cancelling graceCtx first makes
				// the timer callback take its early-exit branch if it is
				// firing right now; stopping the timer covers the case where
				// it has not fired yet.
				abortFn = func() {
					graceCancel()
					timer.Stop()
				}
				continue
			}

			// count > 0 — if a grace timer is pending, a new session
			// arrived during the grace period (D-06). Abort the shutdown.
			if abortFn != nil {
				slog.Info("new session during grace period, cancelling shutdown",
					"sessions", count)
				abortFn()
				abortFn = nil
			}
		}
	}
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

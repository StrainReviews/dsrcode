// Package server — hook_dedup.go
//
// HookDedupMiddleware filters duplicate POST /hooks/* requests arriving
// within hookDedupTTL with identical (route, session_id, tool_name, body)
// tuples. Phase 8 RLC-07..RLC-10, RLC-14. Eliminates the "PreToolUse fires
// 2x within 30-130 ms" symptom documented in Phase 8 trigger logs.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// hookDedupTTL is how long a given request-fingerprint remains in the
	// dedup cache. Tuned from empirical 30-130 ms duplicate spacing
	// observed in Phase 8 trigger logs × 4 safety margin. No legitimate
	// user action produces identical session+tool+body within this window.
	hookDedupTTL = 500 * time.Millisecond

	// hookDedupCleanupPeriod is the cadence of the background GC walk.
	// Larger than TTL — entries may live up to (TTL + CleanupPeriod)
	// before eviction; memory-bounded by rps × CleanupPeriod.
	hookDedupCleanupPeriod = 60 * time.Second

	// hookBodyCap bounds r.Body reads via http.MaxBytesReader.
	// Hook payloads observed in practice: <10 KB. A 64 KiB cap is
	// generous and breaks no legitimate use — see 08-RESEARCH.md
	// §Security Domain "Unbounded io.ReadAll" threat pattern.
	hookBodyCap = 64 << 10 // 65536 bytes

	// dedupSep is the ASCII Unit Separator used to delimit fields when
	// composing the dedup key. 0x1F is reserved for field separation in
	// ASCII and effectively never appears in user content. Matches the
	// coalescer/hash.go separator for cross-file consistency. See
	// 08-RESEARCH.md discrepancy D3 (supersedes CONTEXT.md D-12's
	// null-byte separator).
	dedupSep byte = 0x1F
)

// HookDedupMiddleware filters duplicate POST /hooks/* requests that
// arrive within hookDedupTTL and contain identical (route, session_id,
// tool_name, body) tuples. Stateless w.r.t. the handler — the handler
// sees no difference between a deduped request and a dropped one; only
// the log line differs.
//
// Lifecycle:
//   - Constructed in NewServer (no ctx needed — owns only sync.Map + counter).
//   - StartCleanup(ctx) is called from Server.Start; background goroutine
//     evicts stale entries every hookDedupCleanupPeriod.
//   - No explicit Stop method needed — ctx.Done() drives shutdown.
type HookDedupMiddleware struct {
	seen    sync.Map     // map[string]time.Time — key hex -> last-seen
	deduped atomic.Int64 // total dedup hits since start (non-resetting)
}

// NewHookDedupMiddleware constructs the middleware. Does NOT start the
// cleanup loop — StartCleanup(ctx) must be called separately (we want
// the Server's context to drive eviction lifecycle).
func NewHookDedupMiddleware() *HookDedupMiddleware {
	return &HookDedupMiddleware{}
}

// StartCleanup launches the background GC goroutine. Safe to call once
// per Server.Start lifecycle. Defers Ticker.Stop + defer-recover per
// Phase 6 D-09 precedent.
func (m *HookDedupMiddleware) StartCleanup(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("hook dedup cleanup panic", "panic", r)
			}
		}()
		ticker := time.NewTicker(hookDedupCleanupPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				m.evictOlderThan(now)
			}
		}
	}()
}

// evictOlderThan walks the sync.Map and deletes entries whose timestamp
// is older than hookDedupTTL. Extracted for testability.
func (m *HookDedupMiddleware) evictOlderThan(now time.Time) {
	m.seen.Range(func(k, v any) bool {
		if t, ok := v.(time.Time); ok && now.Sub(t) > hookDedupTTL {
			m.seen.Delete(k)
		}
		return true
	})
}

// DedupedCount returns the total number of dedup hits since the
// middleware was constructed. Non-resetting by design: the Coalescer's
// 60 s summary uses this as the observed cumulative count (delta is
// computed by the caller if needed).
func (m *HookDedupMiddleware) DedupedCount() int64 {
	return m.deduped.Load()
}

// Wrap returns an http.Handler that filters duplicate /hooks/* requests
// and delegates to next for everything else (pass-through).
func (m *HookDedupMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Non-hook routes pass through untouched. Keeps /preview,
		// /status, /health, etc. out of the dedup cache (RLC-07 +
		// RLC-14).
		if !strings.HasPrefix(r.URL.Path, "/hooks/") {
			next.ServeHTTP(w, r)
			return
		}

		// Body cap: defence-in-depth against unbounded io.ReadAll DoS.
		// MaxBytesReader wraps r.Body so ReadAll returns
		// http.MaxBytesError if the client sends >hookBodyCap.
		r.Body = http.MaxBytesReader(w, r.Body, hookBodyCap)
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			// Fail-open: if we couldn't read the body (cap exceeded or
			// client disconnect), pass through to the handler which
			// will return its own 400/413. Do NOT dedup on partial
			// reads — could miss a legitimate subsequent request.
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			next.ServeHTTP(w, r)
			return
		}
		// Re-wrap so downstream handleHook's io.ReadAll sees the same
		// bytes. NopCloser(bytes.NewReader) is idiomatic per 08-RESEARCH
		// §Pattern 7.
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Probe session_id + tool_name from the JSON body. Unmarshal
		// into a tiny tolerant struct; ignore errors (fail-open — an
		// unparseable body still gets a hash over raw bytes + empty
		// probe fields, and the downstream handler returns its own
		// 400 if the JSON is invalid).
		var probe struct {
			SessionID string `json:"session_id"`
			ToolName  string `json:"tool_name"`
		}
		_ = json.Unmarshal(bodyBytes, &probe)

		// FNV-64a key composition. See 08-CONTEXT.md D-12 + RESEARCH
		// discrepancy D3 (0x1F separator chosen over the original null
		// byte proposal for robustness against legitimate NUL bytes in
		// tool payloads).
		h := fnv.New64a()
		h.Write([]byte(r.URL.Path))
		h.Write([]byte{dedupSep})
		h.Write([]byte(probe.SessionID))
		h.Write([]byte{dedupSep})
		h.Write([]byte(probe.ToolName))
		h.Write([]byte{dedupSep})
		h.Write(bodyBytes)
		key := strconv.FormatUint(h.Sum64(), 16)

		now := time.Now()
		if prev, ok := m.seen.Load(key); ok {
			if t, ok := prev.(time.Time); ok && now.Sub(t) < hookDedupTTL {
				// Silent 200 per D-15 — Claude Code sees no difference.
				m.deduped.Add(1)
				slog.Debug("hook deduped", "route", r.URL.Path, "key", shortKey(key))
				w.WriteHeader(http.StatusOK)
				return
			}
			// Older than TTL -> fall through and Store fresh timestamp below.
		}
		m.seen.Store(key, now)
		next.ServeHTTP(w, r)
	})
}

// shortKey truncates a hex key to the first 8 chars for log readability.
// Full key is never logged — ops only needs to correlate dedupes in a
// burst, not reverse-engineer session content.
func shortKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8]
}

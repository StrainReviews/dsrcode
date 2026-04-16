// Package server_test — hook_dedup_test.go
//
// Integration tests for the HookDedupMiddleware introduced in Plan 08-03.
// Covers RLC-07 (path filter), RLC-08 (key composition), RLC-09 (TTL
// cleanup), RLC-10 (body preservation), and RLC-14 (/preview bypass).
//
// Tests go through server.Server.Handler() so the real mux + middleware
// composition is exercised end-to-end; no internal access needed.
package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/StrainReviews/dsrcode/server"
	"github.com/StrainReviews/dsrcode/session"
)

// newDedupTestServer mirrors newTestServer from server_test.go but is
// kept independent so Plan 08-03 changes stay scoped to this file.
// Uses the Server's real Handler() so the full middleware -> mux chain
// is exercised.
func newDedupTestServer(t *testing.T) (*server.Server, *session.SessionRegistry, http.Handler) {
	t.Helper()
	registry := session.NewRegistry(func() {})
	srv := server.NewServer(
		registry,
		func(server.ConfigUpdatePayload) {},
		"8.0.0-test",
		func() server.ServerConfig {
			return server.ServerConfig{
				Preset:        "minimal",
				DisplayDetail: "minimal",
				Port:          19460,
				BindAddr:      "127.0.0.1",
			}
		},
		func() bool { return false },
		nil,
		nil,
	)
	return srv, registry, srv.Handler()
}

// postJSON is a concise helper for the common POST + recorder pattern.
func postJSON(t *testing.T, h http.Handler, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// --- RLC-07 Path filter ---

// TestHookDedup_PathFilter asserts that only /hooks/* routes are
// subject to dedup; all other routes pass through untouched even
// when called repeatedly with identical bodies.
func TestHookDedup_PathFilter(t *testing.T) {
	srv, _, handler := newDedupTestServer(t)

	// Non-hook paths should NEVER be deduped. Issue two POSTs to each;
	// assert deduped counter stays at 0.
	nonHookPaths := []struct {
		path string
		body string
	}{
		{"/statusline", `{"session_id":"s1","cost":0.01}`},
		{"/config", `{"preset":"minimal"}`},
	}
	for _, p := range nonHookPaths {
		postJSON(t, handler, p.path, p.body)
		postJSON(t, handler, p.path, p.body)
	}
	if got := srv.HookDedupedCount(); got != 0 {
		t.Fatalf("non-hook paths should not be deduped; got %d dedup hits", got)
	}

	// Now a /hooks/* path with duplicate body -> MUST dedup once.
	hookBody := `{"session_id":"s1","tool_name":"Edit","cwd":"/tmp"}`
	rec1 := postJSON(t, handler, "/hooks/pre-tool-use", hookBody)
	rec2 := postJSON(t, handler, "/hooks/pre-tool-use", hookBody)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first hook POST: got %d, want 200", rec1.Code)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("deduped hook POST: got %d, want 200", rec2.Code)
	}
	if got := srv.HookDedupedCount(); got != 1 {
		t.Fatalf("expected 1 dedup hit after duplicate /hooks/* POST; got %d", got)
	}
}

// --- RLC-08 Key composition ---

// TestHookDedup_KeyComposition asserts the key is composed of
// (route, session_id, tool_name, body): changing any field (session
// or tool) while the rest stays identical MUST NOT dedup.
//
// Each request carries a distinct cwd so that SessionRegistry's
// per-project upgrade/merge logic (session/registry.go) does not
// collapse the sessions into one — this test probes middleware
// key composition, not registry dedup.
func TestHookDedup_KeyComposition(t *testing.T) {
	srv, _, handler := newDedupTestServer(t)

	// Same route + same tool but DIFFERENT session_id AND body bytes
	// (because cwd differs) -> distinct keys -> no dedup.
	postJSON(t, handler, "/hooks/pre-tool-use", `{"session_id":"s1","tool_name":"Edit","cwd":"/tmp/proj-a"}`)
	postJSON(t, handler, "/hooks/pre-tool-use", `{"session_id":"s2","tool_name":"Edit","cwd":"/tmp/proj-b"}`)
	if got := srv.HookDedupedCount(); got != 0 {
		t.Fatalf("different session_ids must not dedup; got %d hits", got)
	}

	// Same route + same session_id + DIFFERENT tool_name -> distinct keys.
	postJSON(t, handler, "/hooks/pre-tool-use", `{"session_id":"s3","tool_name":"Read","cwd":"/tmp/proj-c"}`)
	postJSON(t, handler, "/hooks/pre-tool-use", `{"session_id":"s3","tool_name":"Edit","cwd":"/tmp/proj-c"}`)
	if got := srv.HookDedupedCount(); got != 0 {
		t.Fatalf("different tool_names must not dedup; got %d hits", got)
	}
}

// --- RLC-09 TTL cleanup ---

// TestHookDedup_TTLCleanup asserts that after the 500 ms TTL window
// elapses, an identical request is NOT deduped — the cached entry
// has expired from the Load-check perspective even if the entry
// still lives in the sync.Map (cleanup goroutine runs every 60 s).
//
// Uses a 600 ms real-time sleep — the 100 ms slack over the 500 ms
// TTL keeps the test fast while guaranteeing TTL expiry.
func TestHookDedup_TTLCleanup(t *testing.T) {
	srv, _, handler := newDedupTestServer(t)

	body := `{"session_id":"s1","tool_name":"Edit","cwd":"/tmp"}`
	postJSON(t, handler, "/hooks/pre-tool-use", body)
	postJSON(t, handler, "/hooks/pre-tool-use", body)
	if got := srv.HookDedupedCount(); got != 1 {
		t.Fatalf("burst of 2 must produce 1 dedup hit; got %d", got)
	}

	// Wait past the TTL.
	time.Sleep(600 * time.Millisecond)

	// Third POST with identical body should NOT be deduped — the prior
	// entry is TTL-expired (middleware Load check fails now-t >= TTL).
	postJSON(t, handler, "/hooks/pre-tool-use", body)
	if got := srv.HookDedupedCount(); got != 1 {
		t.Fatalf("post-TTL identical body must not dedup; got cumulative %d (expected 1)", got)
	}
}

// --- RLC-10 Body preserved for downstream decode ---

// TestHookDedup_BodyPreserved asserts that the middleware rewraps
// r.Body so the downstream handler's io.ReadAll + json.Unmarshal
// still see the original bytes. Observable via registry state
// reflecting the tool_name and creating the session.
func TestHookDedup_BodyPreserved(t *testing.T) {
	_, registry, handler := newDedupTestServer(t)

	body := `{"session_id":"body-preserve-s1","tool_name":"Edit","cwd":"/tmp/myproject"}`
	rec := postJSON(t, handler, "/hooks/pre-tool-use", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Downstream handler must have decoded the body and created a session.
	if registry.SessionCount() != 1 {
		t.Fatalf("expected 1 session after body-preserving middleware; got %d", registry.SessionCount())
	}
	s := registry.GetSession("body-preserve-s1")
	if s == nil {
		t.Fatal("session 'body-preserve-s1' not found; downstream decode did not see the body")
	}
	if s.SmallImageKey != "coding" {
		t.Errorf("expected SmallImageKey 'coding' from Edit tool mapping; got %q", s.SmallImageKey)
	}
}

// --- RLC-14 /preview bypasses dedup middleware ---

// TestPreview_BypassesCoalescer asserts that /preview is NOT subject
// to hook-dedup filtering (per D-22 / RLC-14: preview is user-
// initiated and time-bounded, must not be throttled or deduped).
// /preview is not under the /hooks/ prefix so it falls through the
// path filter.
func TestPreview_BypassesCoalescer(t *testing.T) {
	srv, _, handler := newDedupTestServer(t)

	// POST /preview twice with identical body in rapid succession.
	// /preview is NOT a /hooks/ path -> middleware pass-through -> no dedup.
	body := `{"preset":"minimal","duration_s":5}`
	postJSON(t, handler, "/preview", body)
	postJSON(t, handler, "/preview", body)

	if got := srv.HookDedupedCount(); got != 0 {
		t.Fatalf("/preview must bypass dedup middleware; got %d dedup hits", got)
	}
}

// --- synctest compatibility smoke ---

// hookDedupTTLSlackForTest is slightly larger than the middleware's
// hookDedupTTL to avoid edge-of-window flakiness inside a synctest
// bubble. Must stay > 500 ms.
const hookDedupTTLSlackForTest = 600 * time.Millisecond

// TestHookDedup_SynctestCompat runs the dedup behaviour inside a
// Go 1.25+ synctest bubble with virtualized time. Confirms the
// middleware's time.Now() calls are virtualizable (matches the
// RLC-15 probe outcome from coalescer/coalescer_test.go).
func TestHookDedup_SynctestCompat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv, _, handler := newDedupTestServer(t)
		body := `{"session_id":"st-s1","tool_name":"Edit","cwd":"/tmp"}`
		postJSON(t, handler, "/hooks/pre-tool-use", body)
		postJSON(t, handler, "/hooks/pre-tool-use", body)
		if got := srv.HookDedupedCount(); got != 1 {
			t.Fatalf("burst dedup inside synctest bubble; got %d", got)
		}
		// Advance virtual clock beyond TTL.
		time.Sleep(hookDedupTTLSlackForTest)
		synctest.Wait()
		postJSON(t, handler, "/hooks/pre-tool-use", body)
		if got := srv.HookDedupedCount(); got != 1 {
			t.Fatalf("post-TTL inside bubble must not add dedup hit; got cumulative %d", got)
		}
	})
}

//go:build phase16

package session_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tsanva/cc-discord-presence/session"
)

// TestRegistrySessionDedup_Phase16 contains 8 table-driven subtests covering
// session deduplication with the Source enum upgrade chain, no-downgrade rules,
// same-project-different-PID handling, and Source enum serialization.
//
// All tests reference session.SessionSource and session.Session.Source which do
// NOT exist yet. This file is gated by //go:build phase16 so that normal CI
// (go test ./...) continues to pass. Plan 01 will add the types and remove
// this build tag.
func TestRegistrySessionDedup_Phase16(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "JSONL_then_HTTP_upgrades",
			run: func(t *testing.T) {
				// Start a JSONL-sourced session (PID=0, Source=SourceJSONL),
				// then start an HTTP-sourced session for the same project with
				// a real PID and Source=SourceHTTP. The registry should merge
				// them: SessionCount()=1, surviving session has Source=SourceHTTP,
				// SessionID="http-MyProject", PID=real. StartTime preserved from
				// the JSONL session.
				changed := 0
				reg := session.NewRegistry(func() { changed++ })

				jsonlReq := session.ActivityRequest{
					SessionID: "jsonl-MyProject",
					Cwd:       "/home/user/MyProject",
				}
				s1 := reg.StartSessionWithSource(jsonlReq, 0, session.SourceJSONL)
				if s1 == nil {
					t.Fatal("StartSessionWithSource returned nil for JSONL session")
				}
				originalStart := s1.StartedAt

				// Small delay so we can verify StartTime is preserved, not reset
				time.Sleep(5 * time.Millisecond)

				httpReq := session.ActivityRequest{
					SessionID: "http-MyProject",
					Cwd:       "/home/user/MyProject",
				}
				s2 := reg.StartSessionWithSource(httpReq, 1234, session.SourceHTTP)
				if s2 == nil {
					t.Fatal("StartSessionWithSource returned nil for HTTP session")
				}

				if reg.SessionCount() != 1 {
					t.Errorf("SessionCount = %d, want 1 (JSONL should be upgraded to HTTP)", reg.SessionCount())
				}
				if s2.Source != session.SourceHTTP {
					t.Errorf("Source = %v, want SourceHTTP", s2.Source)
				}
				if s2.SessionID != "http-MyProject" {
					t.Errorf("SessionID = %q, want %q", s2.SessionID, "http-MyProject")
				}
				if s2.PID != 1234 {
					t.Errorf("PID = %d, want 1234", s2.PID)
				}
				if !s2.StartedAt.Equal(originalStart) {
					t.Errorf("StartedAt = %v, want %v (preserved from JSONL)", s2.StartedAt, originalStart)
				}
			},
		},
		{
			name: "HTTP_then_UUID_upgrades",
			run: func(t *testing.T) {
				// Start an HTTP session (PID=real, Source=SourceHTTP), then
				// start a UUID session (PID=real, Source=SourceClaude). The
				// registry should upgrade: SessionCount()=1, surviving session
				// has Source=SourceClaude and the UUID SessionID.
				reg := session.NewRegistry(func() {})

				httpReq := session.ActivityRequest{
					SessionID: "http-MyProject",
					Cwd:       "/home/user/MyProject",
				}
				reg.StartSessionWithSource(httpReq, 5678, session.SourceHTTP)

				uuidReq := session.ActivityRequest{
					SessionID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
					Cwd:       "/home/user/MyProject",
				}
				s := reg.StartSessionWithSource(uuidReq, 5678, session.SourceClaude)

				if reg.SessionCount() != 1 {
					t.Errorf("SessionCount = %d, want 1 (HTTP upgraded to Claude)", reg.SessionCount())
				}
				if s.Source != session.SourceClaude {
					t.Errorf("Source = %v, want SourceClaude", s.Source)
				}
				if s.SessionID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
					t.Errorf("SessionID = %q, want UUID", s.SessionID)
				}
			},
		},
		{
			name: "JSONL_then_UUID_upgrades",
			run: func(t *testing.T) {
				// Full chain: start a JSONL session, then a UUID session for
				// the same project. Expect: SessionCount()=1, Source=SourceClaude.
				reg := session.NewRegistry(func() {})

				jsonlReq := session.ActivityRequest{
					SessionID: "jsonl-SRS",
					Cwd:       "/home/user/SRS",
				}
				reg.StartSessionWithSource(jsonlReq, 0, session.SourceJSONL)

				uuidReq := session.ActivityRequest{
					SessionID: "uuid-real-session",
					Cwd:       "/home/user/SRS",
				}
				s := reg.StartSessionWithSource(uuidReq, 9999, session.SourceClaude)

				if reg.SessionCount() != 1 {
					t.Errorf("SessionCount = %d, want 1 (full chain JSONL->Claude)", reg.SessionCount())
				}
				if s.Source != session.SourceClaude {
					t.Errorf("Source = %v, want SourceClaude", s.Source)
				}
			},
		},
		{
			name: "No_downgrade_UUID_to_JSONL",
			run: func(t *testing.T) {
				// Start a UUID session (SourceClaude), then attempt to start a
				// JSONL session for the same project. The UUID session must NOT
				// be replaced. SessionCount()=1, SessionID=UUID.
				reg := session.NewRegistry(func() {})

				uuidReq := session.ActivityRequest{
					SessionID: "uuid-primary",
					Cwd:       "/home/user/project",
				}
				reg.StartSessionWithSource(uuidReq, 1000, session.SourceClaude)

				jsonlReq := session.ActivityRequest{
					SessionID: "jsonl-project",
					Cwd:       "/home/user/project",
				}
				s := reg.StartSessionWithSource(jsonlReq, 0, session.SourceJSONL)

				if reg.SessionCount() != 1 {
					t.Errorf("SessionCount = %d, want 1 (no downgrade)", reg.SessionCount())
				}
				if s.SessionID != "uuid-primary" {
					t.Errorf("SessionID = %q, want %q (UUID should not be replaced)", s.SessionID, "uuid-primary")
				}
				if s.Source != session.SourceClaude {
					t.Errorf("Source = %v, want SourceClaude (should not downgrade)", s.Source)
				}
			},
		},
		{
			name: "No_downgrade_HTTP_to_JSONL",
			run: func(t *testing.T) {
				// Start an HTTP session (SourceHTTP), then attempt a JSONL
				// session for the same project. No replacement should occur.
				reg := session.NewRegistry(func() {})

				httpReq := session.ActivityRequest{
					SessionID: "http-project",
					Cwd:       "/home/user/project",
				}
				reg.StartSessionWithSource(httpReq, 2000, session.SourceHTTP)

				jsonlReq := session.ActivityRequest{
					SessionID: "jsonl-project",
					Cwd:       "/home/user/project",
				}
				s := reg.StartSessionWithSource(jsonlReq, 0, session.SourceJSONL)

				if reg.SessionCount() != 1 {
					t.Errorf("SessionCount = %d, want 1 (no downgrade)", reg.SessionCount())
				}
				if s.SessionID != "http-project" {
					t.Errorf("SessionID = %q, want %q (HTTP should not be replaced by JSONL)", s.SessionID, "http-project")
				}
				if s.Source != session.SourceHTTP {
					t.Errorf("Source = %v, want SourceHTTP", s.Source)
				}
			},
		},
		{
			name: "Same_project_different_PID_keeps_both",
			run: func(t *testing.T) {
				// Two sessions with different real PIDs on the same project
				// represent two separate Claude Code windows (per D-01).
				// Both should coexist: SessionCount()=2.
				reg := session.NewRegistry(func() {})

				req1 := session.ActivityRequest{
					SessionID: "sess-window-1",
					Cwd:       "/home/user/project",
				}
				reg.StartSessionWithSource(req1, 100, session.SourceClaude)

				req2 := session.ActivityRequest{
					SessionID: "sess-window-2",
					Cwd:       "/home/user/project",
				}
				reg.StartSessionWithSource(req2, 200, session.SourceClaude)

				if reg.SessionCount() != 2 {
					t.Errorf("SessionCount = %d, want 2 (different PIDs = different windows)", reg.SessionCount())
				}
			},
		},
		{
			name: "Source_enum_String",
			run: func(t *testing.T) {
				// Verify that the SessionSource String() method returns the
				// expected human-readable names.
				tests := []struct {
					source session.SessionSource
					want   string
				}{
					{session.SourceJSONL, "jsonl"},
					{session.SourceHTTP, "http"},
					{session.SourceClaude, "claude"},
				}
				for _, tt := range tests {
					got := tt.source.String()
					if got != tt.want {
						t.Errorf("%v.String() = %q, want %q", tt.source, got, tt.want)
					}
				}
			},
		},
		{
			name: "Source_enum_MarshalJSON",
			run: func(t *testing.T) {
				// Verify that json.Marshal on a Session produces "source":"claude"
				// (not a numeric iota value) per D-24.
				reg := session.NewRegistry(func() {})
				req := session.ActivityRequest{
					SessionID: "json-test",
					Cwd:       "/home/user/project",
				}
				s := reg.StartSessionWithSource(req, 42, session.SourceClaude)
				if s == nil {
					t.Fatal("StartSessionWithSource returned nil")
				}

				data, err := json.Marshal(s)
				if err != nil {
					t.Fatalf("json.Marshal failed: %v", err)
				}

				jsonStr := string(data)
				if !strings.Contains(jsonStr, `"source":"claude"`) {
					t.Errorf("JSON = %s, expected to contain %q", jsonStr, `"source":"claude"`)
				}
				// Verify it does NOT contain numeric iota (e.g., "source":2)
				if strings.Contains(jsonStr, `"source":2`) || strings.Contains(jsonStr, `"source":0`) || strings.Contains(jsonStr, `"source":1`) {
					t.Errorf("JSON = %s, should not contain numeric source value", jsonStr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

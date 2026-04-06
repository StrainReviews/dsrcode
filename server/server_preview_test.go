//go:build phase16

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

// TestPreviewEndpoints_Phase16 contains 3 table-driven subtests for enhanced
// preview/demo mode per D-06 to D-12.
//
// Tests 1 and 2 reference PreviewPayload fields (smallImage, fakeSessions) that
// do NOT exist yet. Test 3 hits a GET /preview/messages endpoint that does not
// exist yet (will return 404/405). All are gated by //go:build phase16 so that
// normal CI (go test ./...) continues to pass. Plan 04 will add the types/
// endpoints and remove this build tag.
func TestPreviewEndpoints_Phase16(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Preview_SmallImage",
			run: func(t *testing.T) {
				// POST /preview with smallImage, smallText, and duration.
				// Per D-07: PreviewPayload extended with smallImage and smallText
				// fields for full Discord Activity control in demo mode.
				// Expected: 200 OK with {"ok":true}.
				// Will FAIL until Plan 04 extends PreviewPayload with these fields.
				registry := session.NewRegistry(func() {})
				srv := server.NewServer(
					registry,
					nil,
					"3.1.0",
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
				handler := srv.Handler()

				body := `{
					"smallImage": "coding",
					"smallText": "Editing files",
					"duration": 30
				}`
				req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp struct {
					OK bool `json:"ok"`
				}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if !resp.OK {
					t.Error("expected ok=true in response")
				}

				// Verify the new fields were accepted by the server.
				// The preview callback should have received the smallImage
				// and smallText values. Full verification deferred to Plan 04
				// when the fields are wired through to the Discord activity.
			},
		},
		{
			name: "Preview_FakeSessions",
			run: func(t *testing.T) {
				// POST /preview with fakeSessions array and sessionCount for
				// multi-session demo screenshots per D-07.
				// Expected: 200 OK.
				// Will FAIL until Plan 04 extends PreviewPayload with
				// fakeSessions and sessionCount fields.
				registry := session.NewRegistry(func() {})
				srv := server.NewServer(
					registry,
					nil,
					"3.1.0",
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
				handler := srv.Handler()

				body := `{
					"fakeSessions": [
						{"projectName": "my-saas-app", "model": "Opus 4.6"},
						{"projectName": "api-gateway", "model": "Sonnet 4.6"}
					],
					"sessionCount": 2
				}`
				req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp struct {
					OK bool `json:"ok"`
				}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if !resp.OK {
					t.Error("expected ok=true in response")
				}
			},
		},
		{
			name: "GET_preview_messages",
			run: func(t *testing.T) {
				// GET /preview/messages?preset=minimal&activity=coding&count=3
				// Per D-08: New endpoint that returns an array of Activity
				// objects showing StablePick rotation for different time windows.
				// Expected: 200 OK with JSON array of 3 message objects.
				// Will FAIL until Plan 04 adds the GET /preview/messages handler.
				registry := session.NewRegistry(func() {})
				srv := server.NewServer(
					registry,
					nil,
					"3.1.0",
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
				handler := srv.Handler()

				req := httptest.NewRequest(http.MethodGet, "/preview/messages?preset=minimal&activity=coding&count=3", nil)

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var messages []struct {
					Details    string `json:"details"`
					State      string `json:"state"`
					SmallImage string `json:"smallImage"`
					SmallText  string `json:"smallText"`
				}
				if err := json.NewDecoder(w.Body).Decode(&messages); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(messages) != 3 {
					t.Errorf("expected 3 messages, got %d", len(messages))
				}

				// Each message should have non-empty details and state
				for i, m := range messages {
					if m.Details == "" {
						t.Errorf("message[%d].details is empty", i)
					}
					if m.State == "" {
						t.Errorf("message[%d].state is empty", i)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

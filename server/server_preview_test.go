package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StrainReviews/dsrcode/server"
	"github.com/StrainReviews/dsrcode/session"
)

// TestPreviewEndpoints_Phase16 contains 3 table-driven subtests for enhanced
// preview/demo mode per D-06 to D-12.
//
// Test 1: POST /preview with smallImage field accepted (extended payload)
// Test 2: POST /preview with fakeSessions array accepted (multi-session demo)
// Test 3: GET /preview/messages returns message rotation preview
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
			},
		},
		{
			name: "Preview_FakeSessions",
			run: func(t *testing.T) {
				// POST /preview with fakeSessions array and sessionCount for
				// multi-session demo screenshots per D-07.
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
				// Per D-08: Endpoint returns an array of Activity objects showing
				// StablePick rotation for different time windows.
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
					LargeText  string `json:"largeText"`
					TimeWindow string `json:"timeWindow"`
				}
				if err := json.NewDecoder(w.Body).Decode(&messages); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(messages) != 3 {
					t.Fatalf("expected 3 messages, got %d", len(messages))
				}

				// Each message should have non-empty details, state, and realistic demo data
				for i, m := range messages {
					if m.Details == "" {
						t.Errorf("message[%d].details is empty", i)
					}
					if m.State == "" {
						t.Errorf("message[%d].state is empty", i)
					}
					if m.SmallImage != "coding" {
						t.Errorf("message[%d].smallImage: got %q, want %q", i, m.SmallImage, "coding")
					}
					if m.SmallText == "" {
						t.Errorf("message[%d].smallText is empty", i)
					}
					if m.LargeText == "" {
						t.Errorf("message[%d].largeText is empty", i)
					}
					if m.TimeWindow == "" {
						t.Errorf("message[%d].timeWindow is empty", i)
					}
				}

				// LargeText should contain project name and branch
				if !strings.Contains(messages[0].LargeText, "my-saas-app") {
					t.Errorf("message[0].largeText missing project name: %q", messages[0].LargeText)
				}
				if !strings.Contains(messages[0].LargeText, "feature/auth") {
					t.Errorf("message[0].largeText missing branch: %q", messages[0].LargeText)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

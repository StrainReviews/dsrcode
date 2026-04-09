package config_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/StrainReviews/dsrcode/config"
)

// TestConfigDefaults verifies that LoadConfig() with no flags, env vars, or
// config file returns the expected defaults: Port=19460, BindAddr="127.0.0.1",
// Preset="minimal", IdleTimeout=10min, RemoveTimeout=30min,
// StaleCheckInterval=30s, LogLevel="info".
func TestConfigDefaults(t *testing.T) {
	cfg := config.LoadConfig(0, "", false, false, filepath.Join(t.TempDir(), "nonexistent.json"))

	if cfg.Port != 19460 {
		t.Errorf("Port = %d, want 19460", cfg.Port)
	}
	if cfg.BindAddr != "127.0.0.1" {
		t.Errorf("BindAddr = %q, want \"127.0.0.1\"", cfg.BindAddr)
	}
	if cfg.Preset != "minimal" {
		t.Errorf("Preset = %q, want \"minimal\"", cfg.Preset)
	}
	if cfg.IdleTimeout != 10*time.Minute {
		t.Errorf("IdleTimeout = %v, want 10m", cfg.IdleTimeout)
	}
	if cfg.RemoveTimeout != 30*time.Minute {
		t.Errorf("RemoveTimeout = %v, want 30m", cfg.RemoveTimeout)
	}
	if cfg.StaleCheckInterval != 30*time.Second {
		t.Errorf("StaleCheckInterval = %v, want 30s", cfg.StaleCheckInterval)
	}
	if cfg.ReconnectInterval != 15*time.Second {
		t.Errorf("ReconnectInterval = %v, want 15s", cfg.ReconnectInterval)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want \"info\"", cfg.LogLevel)
	}
	if cfg.DiscordClientID != "" {
		t.Errorf("DiscordClientID = %q, want empty", cfg.DiscordClientID)
	}
}

// TestConfigEnvOverride verifies that setting CC_DISCORD_PORT=19999 in the
// environment causes LoadConfig to return Port=19999.
func TestConfigEnvOverride(t *testing.T) {
	t.Setenv("CC_DISCORD_PORT", "19999")

	cfg := config.LoadConfig(0, "", false, false, filepath.Join(t.TempDir(), "nonexistent.json"))

	if cfg.Port != 19999 {
		t.Errorf("Port = %d, want 19999", cfg.Port)
	}
}

// TestConfigFileOverride verifies that writing a temp JSON config file with
// {"preset":"hacker"} and pointing LoadConfig at it returns Preset="hacker".
func TestConfigFileOverride(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"preset":"hacker"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadConfig(0, "", false, false, cfgPath)

	if cfg.Preset != "hacker" {
		t.Errorf("Preset = %q, want \"hacker\"", cfg.Preset)
	}
}

// TestConfigCLIOverride verifies that a CLI flag --port=18000 overrides both
// the environment variable and the config file value.
func TestConfigCLIOverride(t *testing.T) {
	t.Setenv("CC_DISCORD_PORT", "19999")

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"port":18500}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadConfig(18000, "", false, false, cfgPath)

	if cfg.Port != 18000 {
		t.Errorf("Port = %d, want 18000 (CLI should win over env=19999 and file=18500)", cfg.Port)
	}
}

// TestConfigPriority verifies the full priority chain:
// CLI > Env > File > Defaults.
func TestConfigPriority(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"preset":"hacker"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// All layers set: CLI wins
	t.Setenv("CC_DISCORD_PRESET", "streamer")
	cfg := config.LoadConfig(0, "chaotic", false, false, cfgPath)
	if cfg.Preset != "chaotic" {
		t.Errorf("CLI priority: Preset = %q, want \"chaotic\"", cfg.Preset)
	}

	// No CLI: Env wins
	cfg = config.LoadConfig(0, "", false, false, cfgPath)
	if cfg.Preset != "streamer" {
		t.Errorf("Env priority: Preset = %q, want \"streamer\"", cfg.Preset)
	}

	// No CLI, no env: File wins
	t.Setenv("CC_DISCORD_PRESET", "")
	cfg = config.LoadConfig(0, "", false, false, cfgPath)
	if cfg.Preset != "hacker" {
		t.Errorf("File priority: Preset = %q, want \"hacker\"", cfg.Preset)
	}

	// No CLI, no env, no file: Default wins
	noFile := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg = config.LoadConfig(0, "", false, false, noFile)
	if cfg.Preset != "minimal" {
		t.Errorf("Default priority: Preset = %q, want \"minimal\"", cfg.Preset)
	}
}

// TestConfigWatch verifies that writing a config file, starting WatchConfig,
// then modifying the file causes the onReload callback to fire within 500ms.
func TestConfigWatch(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Write initial config
	if err := os.WriteFile(cfgPath, []byte(`{"preset":"minimal"}`), 0644); err != nil {
		t.Fatal(err)
	}

	reloaded := make(chan config.Config, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := config.WatchConfig(ctx, cfgPath, func(cfg config.Config) {
		select {
		case reloaded <- cfg:
		default:
		}
	}); err != nil {
		t.Fatal(err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(cfgPath, []byte(`{"preset":"hacker"}`), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case cfg := <-reloaded:
		if cfg.Preset != "hacker" {
			t.Errorf("reloaded Preset = %q, want \"hacker\"", cfg.Preset)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("onReload was not called within 500ms")
	}
}

// TestParseDisplayDetail verifies that ParseDisplayDetail correctly parses all 4
// valid values and defaults unknown values to DetailMinimal.
func TestParseDisplayDetail(t *testing.T) {
	tests := []struct {
		input string
		want  config.DisplayDetail
	}{
		{"minimal", config.DetailMinimal},
		{"standard", config.DetailStandard},
		{"verbose", config.DetailVerbose},
		{"private", config.DetailPrivate},
		{"unknown", config.DetailMinimal},
		{"", config.DetailMinimal},
		{"STANDARD", config.DetailMinimal}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := config.ParseDisplayDetail(tt.input)
			if got != tt.want {
				t.Errorf("ParseDisplayDetail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestConfigDisplayDetailFromFile verifies that a JSON config file with
// "displayDetail": "verbose" is loaded as DetailVerbose.
func TestConfigDisplayDetailFromFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"displayDetail":"verbose"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadConfig(0, "", false, false, cfgPath)

	if cfg.DisplayDetail != config.DetailVerbose {
		t.Errorf("DisplayDetail = %q, want %q", cfg.DisplayDetail, config.DetailVerbose)
	}
}

// TestConfigDisplayDetailDefault verifies that loading a config with no
// displayDetail field results in DetailMinimal as the default.
func TestConfigDisplayDetailDefault(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"preset":"hacker"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadConfig(0, "", false, false, cfgPath)

	if cfg.DisplayDetail != config.DetailMinimal {
		t.Errorf("DisplayDetail = %q, want %q", cfg.DisplayDetail, config.DetailMinimal)
	}
}

// TestConfigDisplayDetailEnvOverride verifies that the CC_DISCORD_DISPLAY_DETAIL
// environment variable overrides the config file value.
func TestConfigDisplayDetailEnvOverride(t *testing.T) {
	t.Setenv("CC_DISCORD_DISPLAY_DETAIL", "standard")

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"displayDetail":"verbose"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadConfig(0, "", false, false, cfgPath)

	if cfg.DisplayDetail != config.DetailStandard {
		t.Errorf("DisplayDetail = %q, want %q (env should override file)", cfg.DisplayDetail, config.DetailStandard)
	}
}

// TestConfigWatchDebounce verifies that 5 rapid file writes within 50ms
// trigger only a single onReload callback invocation.
func TestConfigWatchDebounce(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Write initial config
	if err := os.WriteFile(cfgPath, []byte(`{"preset":"minimal"}`), 0644); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := config.WatchConfig(ctx, cfgPath, func(cfg config.Config) {
		count.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// 5 rapid writes
	for i := 0; i < 5; i++ {
		data := fmt.Sprintf(`{"preset":"test-%d"}`, i)
		if err := os.WriteFile(cfgPath, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to settle (100ms debounce + margin)
	time.Sleep(300 * time.Millisecond)

	got := count.Load()
	if got != 1 {
		t.Errorf("onReload called %d times, want exactly 1 (debounce should collapse rapid writes)", got)
	}
}

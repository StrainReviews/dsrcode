package config_test

import (
	"testing"
)

// TestConfigDefaults verifies that LoadConfig() with no flags, env vars, or
// config file returns the expected defaults: Port=19460, BindAddr="127.0.0.1",
// Preset="minimal", IdleTimeout=10min, RemoveTimeout=30min,
// StaleCheckInterval=30s, LogLevel="info".
func TestConfigDefaults(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestConfigEnvOverride verifies that setting CC_DISCORD_PORT=19999 in the
// environment causes LoadConfig to return Port=19999.
func TestConfigEnvOverride(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestConfigFileOverride verifies that writing a temp JSON config file with
// {"preset":"hacker"} and pointing LoadConfig at it returns Preset="hacker".
func TestConfigFileOverride(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestConfigCLIOverride verifies that a CLI flag --port=18000 overrides both
// the environment variable and the config file value.
func TestConfigCLIOverride(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestConfigPriority verifies the full priority chain:
// CLI > Env > File > Defaults.
func TestConfigPriority(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestConfigWatch verifies that writing a config file, starting WatchConfig,
// then modifying the file causes the onReload callback to fire within 500ms.
func TestConfigWatch(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestConfigWatchDebounce verifies that 5 rapid file writes within 50ms
// trigger only a single onReload callback invocation.
func TestConfigWatchDebounce(t *testing.T) {
	t.Skip("not implemented yet")
}

// Package config provides runtime configuration loading with a priority chain:
// CLI flags > environment variables > JSON config file > compiled defaults.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// FeatureMap controls which optional features are enabled.
// All features default to true per D-34.
type FeatureMap struct {
	Analytics bool `json:"analytics"`
}

// Config holds all runtime configuration for the daemon.
type Config struct {
	DiscordClientID    string        `json:"discordClientId"`
	Preset             string        `json:"preset"`
	Port               int           `json:"port"`
	BindAddr           string        `json:"bindAddr"`
	IdleTimeout        time.Duration `json:"idleTimeout"`
	RemoveTimeout      time.Duration `json:"removeTimeout"`
	StaleCheckInterval time.Duration `json:"staleCheckInterval"`
	ReconnectInterval  time.Duration `json:"reconnectInterval"`
	LogLevel           string        `json:"logLevel"`
	LogFile            string        `json:"logFile"`
	DisplayDetail      DisplayDetail `json:"displayDetail"`
	Buttons            []Button      `json:"buttons,omitempty"`
	Lang               string        `json:"lang"`     // "en" or "de", default "en" per D-27
	Features           FeatureMap    `json:"features"` // D-34 feature toggles
}

// Button represents a clickable button shown on the Discord Rich Presence activity.
type Button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// DisplayDetail controls how much information is shown in Discord presence.
// Presets determine the tone; DisplayDetail determines the data level.
type DisplayDetail string

const (
	// DetailMinimal shows project name only, hides file/command/query details.
	DetailMinimal DisplayDetail = "minimal"
	// DetailStandard shows file names, truncated commands, and search patterns.
	DetailStandard DisplayDetail = "standard"
	// DetailVerbose shows full relative paths, full commands, and full project names.
	DetailVerbose DisplayDetail = "verbose"
	// DetailPrivate hides all identifying information (file, project, branch, tokens, cost).
	DetailPrivate DisplayDetail = "private"
)

// ParseDisplayDetail converts a string to a DisplayDetail value.
// Unknown values fall back to DetailMinimal.
func ParseDisplayDetail(s string) DisplayDetail {
	switch s {
	case "standard":
		return DetailStandard
	case "verbose":
		return DetailVerbose
	case "private":
		return DetailPrivate
	default:
		return DetailMinimal
	}
}

// fileFeatureMap mirrors the JSON shape for feature toggles.
// Pointer fields allow distinguishing "not set" from "set to false".
type fileFeatureMap struct {
	Analytics *bool `json:"analytics,omitempty"`
}

// fileConfig mirrors the JSON config file shape. Duration fields accept either
// an integer (seconds) or a Go duration string like "10m".
type fileConfig struct {
	DiscordClientID    string           `json:"discordClientId,omitempty"`
	Preset             string           `json:"preset,omitempty"`
	Port               int              `json:"port,omitempty"`
	BindAddr           string           `json:"bindAddr,omitempty"`
	IdleTimeout        *durationOrInt   `json:"idleTimeout,omitempty"`
	RemoveTimeout      *durationOrInt   `json:"removeTimeout,omitempty"`
	StaleCheckInterval *durationOrInt   `json:"staleCheckInterval,omitempty"`
	ReconnectInterval  *durationOrInt   `json:"reconnectInterval,omitempty"`
	LogLevel           string           `json:"logLevel,omitempty"`
	LogFile            string           `json:"logFile,omitempty"`
	DisplayDetail      string           `json:"displayDetail,omitempty"`
	Buttons            []Button         `json:"buttons,omitempty"`
	Lang               string           `json:"lang,omitempty"`
	Features           *fileFeatureMap  `json:"features,omitempty"`
}

// durationOrInt handles JSON values that can be either an integer (seconds)
// or a string duration like "10m", "30s".
type durationOrInt struct {
	Duration time.Duration
}

func (d *durationOrInt) UnmarshalJSON(b []byte) error {
	// Try string first ("10m", "30s")
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", s, err)
		}
		d.Duration = parsed
		return nil
	}

	// Try integer (seconds)
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		d.Duration = time.Duration(n) * time.Second
		return nil
	}

	// Try float (seconds)
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		d.Duration = time.Duration(f * float64(time.Second))
		return nil
	}

	return fmt.Errorf("duration must be a string (\"10m\") or integer (seconds), got %s", string(b))
}

// DefaultConfigPath returns the default location for the config file:
// ~/.claude/discord-presence-config.json
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "discord-presence-config.json"
	}
	return filepath.Join(home, ".claude", "discord-presence-config.json")
}

// defaultLogFile returns the default log file path:
// ~/.claude/discord-presence.log
func defaultLogFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "discord-presence.log"
	}
	return filepath.Join(home, ".claude", "discord-presence.log")
}

// Defaults returns a Config populated with compiled default values per D-29/D-31/D-35/D-55.
func Defaults() Config {
	return Config{
		DiscordClientID:    "",
		Preset:             "minimal",
		Port:               19460,
		BindAddr:           "127.0.0.1",
		IdleTimeout:        10 * time.Minute,
		RemoveTimeout:      30 * time.Minute,
		StaleCheckInterval: 30 * time.Second,
		ReconnectInterval:  15 * time.Second,
		LogLevel:           "info",
		LogFile:            defaultLogFile(),
		DisplayDetail:      DetailMinimal,
		Lang:               "en",
		Features:           FeatureMap{Analytics: true},
	}
}

// LoadConfig builds a Config using the priority chain:
//  1. Compiled defaults
//  2. JSON config file at configPath (or DefaultConfigPath if empty)
//  3. Environment variables (CC_DISCORD_*)
//  4. CLI flags (only applied when non-zero / non-empty)
//
// The -v flag sets LogLevel="debug", -q flag sets LogLevel="error".
func LoadConfig(flagPort int, flagPreset string, flagVerbose bool, flagQuiet bool, configPath string) Config {
	cfg := Defaults()

	// --- Layer 2: JSON config file ---
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	if fc, err := LoadConfigFile(configPath); err == nil {
		applyFileConfig(&cfg, fc)
	}

	// --- Layer 3: Environment variables ---
	applyEnvVars(&cfg)

	// --- Layer 4: CLI flags (only non-zero/non-empty) ---
	if flagPort != 0 {
		cfg.Port = flagPort
	}
	if flagPreset != "" {
		cfg.Preset = flagPreset
	}
	if flagVerbose {
		cfg.LogLevel = "debug"
	}
	if flagQuiet {
		cfg.LogLevel = "error"
	}

	return cfg
}

// LoadConfigFile reads and parses the JSON config file at the given path.
// Returns the parsed fileConfig or an error if the file cannot be read/parsed.
func LoadConfigFile(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &fc, nil
}

// applyFileConfig merges non-zero values from a fileConfig into the Config.
func applyFileConfig(cfg *Config, fc *fileConfig) {
	if fc.DiscordClientID != "" {
		cfg.DiscordClientID = fc.DiscordClientID
	}
	if fc.Preset != "" {
		cfg.Preset = fc.Preset
	}
	if fc.Port != 0 {
		cfg.Port = fc.Port
	}
	if fc.BindAddr != "" {
		cfg.BindAddr = fc.BindAddr
	}
	if fc.IdleTimeout != nil {
		cfg.IdleTimeout = fc.IdleTimeout.Duration
	}
	if fc.RemoveTimeout != nil {
		cfg.RemoveTimeout = fc.RemoveTimeout.Duration
	}
	if fc.StaleCheckInterval != nil {
		cfg.StaleCheckInterval = fc.StaleCheckInterval.Duration
	}
	if fc.ReconnectInterval != nil {
		cfg.ReconnectInterval = fc.ReconnectInterval.Duration
	}
	if fc.LogLevel != "" {
		cfg.LogLevel = fc.LogLevel
	}
	if fc.LogFile != "" {
		cfg.LogFile = fc.LogFile
	}
	if fc.DisplayDetail != "" {
		cfg.DisplayDetail = ParseDisplayDetail(fc.DisplayDetail)
	}
	if len(fc.Buttons) > 0 {
		cfg.Buttons = fc.Buttons
	}
	if fc.Lang != "" {
		cfg.Lang = fc.Lang
	}
	if fc.Features != nil {
		if fc.Features.Analytics != nil {
			cfg.Features.Analytics = *fc.Features.Analytics
		}
	}
}

// applyEnvVars overrides Config fields from CC_DISCORD_* environment variables.
func applyEnvVars(cfg *Config) {
	if v := os.Getenv("CC_DISCORD_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("CC_DISCORD_PRESET"); v != "" {
		cfg.Preset = v
	}
	if v := os.Getenv("CC_DISCORD_BIND_ADDR"); v != "" {
		cfg.BindAddr = v
	}
	if v := os.Getenv("CC_DISCORD_CLIENT_ID"); v != "" {
		cfg.DiscordClientID = v
	}
	if v := os.Getenv("CC_DISCORD_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("CC_DISCORD_DISPLAY_DETAIL"); v != "" {
		cfg.DisplayDetail = ParseDisplayDetail(v)
	}
	if v := os.Getenv("CC_DISCORD_LANG"); v != "" {
		cfg.Lang = v
	}
}

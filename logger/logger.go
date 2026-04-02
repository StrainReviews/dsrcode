// Package logger provides structured logging backed by slog with automatic
// file rotation via lumberjack. Log output goes to both a rotating file and
// stderr so daemon operators see messages in real time while retaining a
// persistent log.
package logger

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	mu            sync.Mutex
	currentLogger *slog.Logger
	currentLevel  *slog.LevelVar
	currentFile   string
	ljWriter      *lumberjack.Logger
)

// Setup initialises the global slog logger with file rotation and stderr output.
//
// Parameters:
//   - logFile: absolute path to the log file (e.g. ~/.claude/discord-presence.log)
//   - level: one of "debug", "info", "error" (default: "info")
//
// Per D-52: 5 MB max file size, rotated on overflow, 2 backups kept for 7 days.
func Setup(logFile string, level string) {
	mu.Lock()
	defer mu.Unlock()

	ljWriter = &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    5, // megabytes per D-52
		MaxBackups: 2,
		MaxAge:     7, // days
		Compress:   false,
	}

	currentLevel = &slog.LevelVar{}
	currentLevel.Set(parseLevel(level))
	currentFile = logFile

	writer := io.MultiWriter(ljWriter, os.Stderr)
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: currentLevel,
	})

	currentLogger = slog.New(handler)
	slog.SetDefault(currentLogger)
}

// SetLevel changes the log level at runtime without re-creating the handler.
// This is safe to call from the config hot-reload callback.
func SetLevel(level string) {
	mu.Lock()
	defer mu.Unlock()

	if currentLevel != nil {
		currentLevel.Set(parseLevel(level))
	}
}

// Get returns the package-level logger. If Setup has not been called yet it
// returns slog.Default() so callers never get nil.
func Get() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	if currentLogger != nil {
		return currentLogger
	}
	return slog.Default()
}

// parseLevel converts a string level name to a slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "error":
		return slog.LevelError
	case "warn":
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

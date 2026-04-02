package config

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceDelay = 100 * time.Millisecond

// WatchConfig watches the config file at path for changes and calls onReload
// with a freshly-loaded Config whenever the file is modified.
//
// It watches the parent directory (not the file itself) because editors that
// perform atomic saves (write-to-temp then rename) would break a file-level
// watch. Only events matching the target filename trigger a reload.
//
// A 100ms debounce timer collapses rapid successive writes into a single
// onReload invocation. The goroutine exits when ctx is cancelled.
//
// Returns an error only if the fsnotify watcher cannot be created or the
// directory cannot be watched.
func WatchConfig(ctx context.Context, path string, onReload func(Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return err
	}

	go runWatchLoop(ctx, watcher, base, path, onReload)

	return nil
}

// runWatchLoop is the internal event loop for the config watcher. It runs in
// its own goroutine and exits when ctx is cancelled.
func runWatchLoop(ctx context.Context, watcher *fsnotify.Watcher, base, path string, onReload func(Config)) {
	defer watcher.Close()

	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only react to the target config file.
			if filepath.Base(event.Name) != base {
				continue
			}

			// Only react to write/create/rename events (not chmod).
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
				continue
			}

			// Reset debounce timer on every matching event.
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				fc, err := LoadConfigFile(path)
				if err != nil {
					slog.Warn("config reload failed", "error", err)
					return
				}
				cfg := Defaults()
				applyFileConfig(&cfg, fc)
				applyEnvVars(&cfg)
				onReload(cfg)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Error("config watcher error", "error", err)
		}
	}
}

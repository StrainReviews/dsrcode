package analytics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// TranscriptResult holds parsed JSONL transcript data extracted by
// ParseTranscript. It is a passive snapshot — the caller decides whether to
// merge it into a Tracker, push it to the registry, or simply log it.
type TranscriptResult struct {
	// Tokens is the per-model token breakdown accumulated across every
	// "assistant" message in the transcript.
	Tokens map[string]TokenBreakdown
	// CompactionCount counts the number of isCompactSummary entries seen.
	CompactionCount int
	// LastModel is the model ID of the most recent assistant message
	// (used to display the active model in Discord presence).
	LastModel string
	// ProjectPath is the first non-empty cwd field encountered (used to
	// derive project name and git branch).
	ProjectPath string
}

// transcriptMessage mirrors the Claude Code JSONL line schema. It is
// intentionally unexported — the public surface is TranscriptResult only.
type transcriptMessage struct {
	Type             string `json:"type"`
	Timestamp        string `json:"timestamp"`
	Cwd              string `json:"cwd"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	UUID             string `json:"uuid"`
	Message          struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParseTranscript reads a JSONL transcript file and extracts per-model token
// breakdowns and compaction count. It is the foundation for hook-triggered
// reads (D-01, D-02): callers — including PostToolUse, PreCompact and
// SessionEnd handlers — invoke ParseTranscript with the transcript_path from
// the hook payload.
//
// An empty path returns a zero-value TranscriptResult and nil error so callers
// can pass through hook payloads that omit transcript_path without extra
// branching.
//
// Per D-02, internal throttling (max 1 read per 10s) is the caller's
// responsibility. ParseTranscript is a pure read function with no rate
// limiting or caching of its own.
//
// Malformed JSON lines are skipped (corruption-tolerant). The 1 MB scanner
// buffer matches the legacy main.go parseJSONLSession implementation and
// must remain explicit — the default 64 KB token size silently truncates
// large transcripts (see github.com/golang/go#26431).
func ParseTranscript(path string) (TranscriptResult, error) {
	if path == "" {
		return TranscriptResult{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return TranscriptResult{}, fmt.Errorf("open transcript %q: %w", path, err)
	}
	defer file.Close()

	result := TranscriptResult{
		Tokens: make(map[string]TokenBreakdown),
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1 MB matches legacy parseJSONLSession

	for scanner.Scan() {
		var msg transcriptMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			// Skip malformed lines (resilient against partial writes
			// and concurrent appends from Claude Code).
			continue
		}

		if msg.Cwd != "" && result.ProjectPath == "" {
			result.ProjectPath = msg.Cwd
		}

		if msg.IsCompactSummary {
			result.CompactionCount++
		}

		if msg.Type == "assistant" && msg.Message.Model != "" {
			result.LastModel = msg.Message.Model

			existing := result.Tokens[msg.Message.Model]
			result.Tokens[msg.Message.Model] = TokenBreakdown{
				Input:      existing.Input + msg.Message.Usage.InputTokens,
				Output:     existing.Output + msg.Message.Usage.OutputTokens,
				CacheRead:  existing.CacheRead + msg.Message.Usage.CacheReadInputTokens,
				CacheWrite: existing.CacheWrite + msg.Message.Usage.CacheCreationInputTokens,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return TranscriptResult{}, fmt.Errorf("scan transcript %q: %w", path, err)
	}

	return result, nil
}

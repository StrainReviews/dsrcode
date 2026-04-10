package analytics_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StrainReviews/dsrcode/analytics"
)

// writeTranscript creates a temp JSONL file containing the given lines and
// returns its path. The temp directory is cleaned up automatically by t.
func writeTranscript(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp transcript: %v", err)
	}
	return path
}

// TestParseTranscript_EmptyPath verifies that an empty path returns a zero
// result with no error so callers can pass through unset transcript_path
// fields from hook payloads without branching.
func TestParseTranscript_EmptyPath(t *testing.T) {
	got, err := analytics.ParseTranscript("")
	if err != nil {
		t.Fatalf("expected nil error for empty path, got %v", err)
	}
	if got.LastModel != "" {
		t.Errorf("LastModel = %q, want empty", got.LastModel)
	}
	if got.CompactionCount != 0 {
		t.Errorf("CompactionCount = %d, want 0", got.CompactionCount)
	}
	if got.ProjectPath != "" {
		t.Errorf("ProjectPath = %q, want empty", got.ProjectPath)
	}
	if len(got.Tokens) != 0 {
		t.Errorf("Tokens len = %d, want 0", len(got.Tokens))
	}
}

// TestParseTranscript_NonexistentFile verifies that a missing file produces
// a wrapped error so handlers can log it.
func TestParseTranscript_NonexistentFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	_, err := analytics.ParseTranscript(missing)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "open transcript") {
		t.Errorf("error message %q does not mention 'open transcript'", err.Error())
	}
}

// TestParseTranscript_EmptyFile verifies that an empty file is not an error.
func TestParseTranscript_EmptyFile(t *testing.T) {
	path := writeTranscript(t, nil)
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.LastModel != "" {
		t.Errorf("LastModel = %q, want empty", got.LastModel)
	}
	if len(got.Tokens) != 0 {
		t.Errorf("Tokens len = %d, want 0", len(got.Tokens))
	}
}

// TestParseTranscript_SingleAssistantMessage verifies basic single-message
// extraction including token fields and project path.
func TestParseTranscript_SingleAssistantMessage(t *testing.T) {
	path := writeTranscript(t, []string{
		`{"type":"user","cwd":"/home/u/proj","timestamp":"2026-04-10T10:00:00Z","message":{}}`,
		`{"type":"assistant","cwd":"/home/u/proj","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":200,"cache_creation_input_tokens":30}}}`,
	})
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProjectPath != "/home/u/proj" {
		t.Errorf("ProjectPath = %q, want /home/u/proj", got.ProjectPath)
	}
	if got.LastModel != "claude-opus-4-6" {
		t.Errorf("LastModel = %q, want claude-opus-4-6", got.LastModel)
	}
	if got.CompactionCount != 0 {
		t.Errorf("CompactionCount = %d, want 0", got.CompactionCount)
	}
	tb := got.Tokens["claude-opus-4-6"]
	want := analytics.TokenBreakdown{Input: 100, Output: 50, CacheRead: 200, CacheWrite: 30}
	if tb != want {
		t.Errorf("Tokens[opus] = %+v, want %+v", tb, want)
	}
}

// TestParseTranscript_PerModelAccumulation verifies that multiple assistant
// messages with the same model accumulate correctly across all four token
// fields, including cache_read and cache_creation.
func TestParseTranscript_PerModelAccumulation(t *testing.T) {
	path := writeTranscript(t, []string{
		`{"type":"assistant","cwd":"/proj","message":{"model":"opus","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":200,"cache_creation_input_tokens":30}}}`,
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":20,"cache_creation_input_tokens":3}}}`,
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":1,"output_tokens":2,"cache_read_input_tokens":3,"cache_creation_input_tokens":4}}}`,
	})
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := analytics.TokenBreakdown{Input: 111, Output: 57, CacheRead: 223, CacheWrite: 37}
	if got.Tokens["opus"] != want {
		t.Errorf("Tokens[opus] = %+v, want %+v", got.Tokens["opus"], want)
	}
}

// TestParseTranscript_MultipleModels verifies that distinct models accumulate
// in separate map entries and that LastModel reflects the most recent.
func TestParseTranscript_MultipleModels(t *testing.T) {
	path := writeTranscript(t, []string{
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":100,"output_tokens":50}}}`,
		`{"type":"assistant","message":{"model":"sonnet","usage":{"input_tokens":200,"output_tokens":80}}}`,
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":50,"output_tokens":25}}}`,
	})
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.LastModel != "opus" {
		t.Errorf("LastModel = %q, want opus", got.LastModel)
	}
	if got.Tokens["opus"].Input != 150 || got.Tokens["opus"].Output != 75 {
		t.Errorf("Tokens[opus] = %+v, want input=150 output=75", got.Tokens["opus"])
	}
	if got.Tokens["sonnet"].Input != 200 || got.Tokens["sonnet"].Output != 80 {
		t.Errorf("Tokens[sonnet] = %+v, want input=200 output=80", got.Tokens["sonnet"])
	}
}

// TestParseTranscript_CompactionCount verifies that isCompactSummary entries
// increment the counter regardless of message type.
func TestParseTranscript_CompactionCount(t *testing.T) {
	path := writeTranscript(t, []string{
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":10}}}`,
		`{"type":"assistant","isCompactSummary":true,"message":{"model":"opus","usage":{}}}`,
		`{"type":"user","isCompactSummary":true,"message":{}}`,
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":20}}}`,
	})
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CompactionCount != 2 {
		t.Errorf("CompactionCount = %d, want 2", got.CompactionCount)
	}
}

// TestParseTranscript_MalformedLinesSkipped verifies corruption tolerance:
// malformed JSON lines are skipped without aborting the scan.
func TestParseTranscript_MalformedLinesSkipped(t *testing.T) {
	path := writeTranscript(t, []string{
		`not json at all`,
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":100}}}`,
		`{broken json`,
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":50}}}`,
	})
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Tokens["opus"].Input != 150 {
		t.Errorf("Tokens[opus].Input = %d, want 150", got.Tokens["opus"].Input)
	}
}

// TestParseTranscript_ProjectPathFirstWins verifies that ProjectPath is
// captured from the first non-empty cwd and not overwritten later.
func TestParseTranscript_ProjectPathFirstWins(t *testing.T) {
	path := writeTranscript(t, []string{
		`{"type":"user","cwd":"/first/path","message":{}}`,
		`{"type":"assistant","cwd":"/second/path","message":{"model":"opus","usage":{}}}`,
	})
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProjectPath != "/first/path" {
		t.Errorf("ProjectPath = %q, want /first/path", got.ProjectPath)
	}
}

// TestParseTranscript_NonAssistantMessagesIgnored verifies that user/system
// messages do not contribute to token totals or LastModel.
func TestParseTranscript_NonAssistantMessagesIgnored(t *testing.T) {
	path := writeTranscript(t, []string{
		`{"type":"user","message":{"model":"opus","usage":{"input_tokens":1000}}}`,
		`{"type":"system","message":{"model":"opus","usage":{"input_tokens":2000}}}`,
		`{"type":"assistant","message":{"model":"opus","usage":{"input_tokens":50}}}`,
	})
	got, err := analytics.ParseTranscript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Tokens["opus"].Input != 50 {
		t.Errorf("Tokens[opus].Input = %d, want 50 (user/system ignored)", got.Tokens["opus"].Input)
	}
}

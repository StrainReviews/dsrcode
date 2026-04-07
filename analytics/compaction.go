package analytics

import "encoding/json"

// jsonlEntry represents a single line from a Claude Code JSONL transcript.
// Fields match the JSONL schema: type, isCompactSummary, uuid, and nested
// message.model + message.usage.
type jsonlEntry struct {
	Type             string `json:"type"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	UUID             string `json:"uuid"`
	Message          struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens           int64 `json:"input_tokens"`
			OutputTokens          int64 `json:"output_tokens"`
			CacheReadInputTokens  int64 `json:"cache_read_input_tokens"`
			CacheCreationTokens   int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParseJSONLEntry unmarshals a single JSONL line into a jsonlEntry.
// Returns an error for malformed JSON (T-17-01-02: skip malformed lines).
func ParseJSONLEntry(line []byte) (jsonlEntry, error) {
	var entry jsonlEntry
	err := json.Unmarshal(line, &entry)
	return entry, err
}

// DetectCompaction checks whether a raw JSONL line represents a compaction
// summary. Only assistant-type entries with isCompactSummary=true qualify.
func DetectCompaction(line []byte) bool {
	entry, err := ParseJSONLEntry(line)
	if err != nil {
		return false
	}
	return entry.Type == "assistant" && entry.IsCompactSummary
}

// ComputeBaseline calculates the baseline accumulation when compaction resets
// token counts in the JSONL file. Per D-19: when current.Input < stored.Input,
// the stored values become the new baseline (they represent tokens lost to
// compaction). Otherwise the baseline stays at zero.
func ComputeBaseline(stored, current TokenBreakdown) TokenBreakdown {
	if stored.Input == 0 && stored.Output == 0 {
		return TokenBreakdown{}
	}
	if current.Input < stored.Input {
		return stored
	}
	return TokenBreakdown{}
}

// EffectiveTotal returns the effective token count: current + baseline.
// This recovers the true total after compaction resets.
func EffectiveTotal(current, baseline TokenBreakdown) TokenBreakdown {
	return TokenBreakdown{
		Input:      current.Input + baseline.Input,
		Output:     current.Output + baseline.Output,
		CacheRead:  current.CacheRead + baseline.CacheRead,
		CacheWrite: current.CacheWrite + baseline.CacheWrite,
	}
}

// CompactionTracker tracks compaction events by UUID for deduplication.
type CompactionTracker struct {
	seen map[string]struct{}
}

// NewCompactionTracker creates a new CompactionTracker with an empty UUID set.
func NewCompactionTracker() *CompactionTracker {
	return &CompactionTracker{
		seen: make(map[string]struct{}),
	}
}

// Record registers a compaction UUID. Duplicate UUIDs are ignored.
func (ct *CompactionTracker) Record(uuid string) {
	ct.seen[uuid] = struct{}{}
}

// Count returns the number of unique compaction events recorded.
func (ct *CompactionTracker) Count() int {
	return len(ct.seen)
}

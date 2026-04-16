package coalescer

import (
	"hash/fnv"

	"github.com/StrainReviews/dsrcode/discord"
)

// sep is the ASCII Unit Separator (0x1F). It is reserved in the ASCII
// control-code block specifically for delimiting fields — it effectively
// never appears in user content. See 08-RESEARCH.md §Pattern 4 +
// discrepancy D3 (supersedes 08-CONTEXT.md D-12, which originally
// specified \x00; 0x1F is the correct choice because (a) 0x1F is reserved
// for field delimiters and will not appear in Details/State/Button URL
// content, and (b) \x00 can break terminal or log handling downstream).
const sep byte = 0x1F

// HashActivity produces a stable 64-bit FNV-1a hash of the user-visible
// fields of an Activity. Used by the Coalescer to skip Discord
// SetActivity calls when the content has not changed since the last
// successful push.
//
// StartTime is intentionally EXCLUDED from the hash (08-CONTEXT.md D-09):
// it is deterministic per-session (the earliest active session's
// StartedAt, verified via resolver/resolver_test.go:333-339), but the
// pointer identity and monotonic-wall offset can produce different byte
// representations for the same logical instant. Including it would
// trigger a spurious hash difference on every resolve cycle.
//
// Serialization is byte-level and field-ordered (no json.Marshal, no
// fmt.Sprintf) so the hash is stable across Go versions — map-ordering
// changes in encoding/json cannot shift the output (D-11).
//
// FNV-1a is NOT cryptographic. It is a content-identity hash only — a
// 64-bit collision is possible but vanishingly rare (~2^32 birthday
// bound) and the worst outcome is one legitimate update skipped; the
// next distinct resolve auto-recovers.
func HashActivity(a *discord.Activity) uint64 {
	if a == nil {
		return 0
	}
	h := fnv.New64a()
	// Write each user-visible field followed by a Unit Separator. The
	// order here is the struct declaration order from discord/client.go
	// lines 25-33 (minus StartTime, which is excluded). Any change to
	// this order silently alters every hash — treat it as frozen.
	h.Write([]byte(a.Details))
	h.Write([]byte{sep})
	h.Write([]byte(a.State))
	h.Write([]byte{sep})
	h.Write([]byte(a.LargeImage))
	h.Write([]byte{sep})
	h.Write([]byte(a.LargeText))
	h.Write([]byte{sep})
	h.Write([]byte(a.SmallImage))
	h.Write([]byte{sep})
	h.Write([]byte(a.SmallText))
	h.Write([]byte{sep})
	// Buttons preserve insertion order (slice), so iteration is
	// deterministic by construction. The resolver never reorders them.
	for _, b := range a.Buttons {
		h.Write([]byte(b.Label))
		h.Write([]byte{sep})
		h.Write([]byte(b.URL))
		h.Write([]byte{sep})
	}
	return h.Sum64()
}

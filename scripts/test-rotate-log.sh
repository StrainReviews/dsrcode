#!/bin/bash
# Test harness for Phase 7 Bug #4 rotate_log function.
# Validates: 11MB file gets renamed to .log.1; second 11MB run overwrites .log.1.
# Usage: bash scripts/test-rotate-log.sh
# Exits 0 on success, non-zero on any assertion failure.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
START_SH="$SCRIPT_DIR/start.sh"

if [[ ! -f "$START_SH" ]]; then
    echo "FAIL: $START_SH not found" >&2
    exit 1
fi

# Source rotate_log function from start.sh by extracting just its definition.
# We can't `source $START_SH` because that runs the entire start sequence.
ROTATE_LOG_DEF=$(awk '/^rotate_log\(\)/,/^}$/' "$START_SH")
if [[ -z "$ROTATE_LOG_DEF" ]]; then
    echo "FAIL: rotate_log function not found in $START_SH" >&2
    exit 1
fi

# Eval the function definition into the current shell.
eval "$ROTATE_LOG_DEF"

# Set up tmp test directory.
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

TESTLOG="$TMPDIR/test.log"

# Test 1: rotate_log on a 5MB file (below threshold) — should NOT rotate.
dd if=/dev/zero of="$TESTLOG" bs=1024 count=5120 status=none
rotate_log "$TESTLOG"
if [[ -f "$TESTLOG.1" ]]; then
    echo "FAIL: 5MB file should NOT have been rotated" >&2
    exit 1
fi
if [[ ! -f "$TESTLOG" ]]; then
    echo "FAIL: 5MB file should still exist" >&2
    exit 1
fi
echo "PASS: 5MB file not rotated"

# Test 2: rotate_log on an 11MB file — should rotate to .log.1.
dd if=/dev/zero of="$TESTLOG" bs=1024 count=11264 status=none
rotate_log "$TESTLOG"
if [[ ! -f "$TESTLOG.1" ]]; then
    echo "FAIL: 11MB file should have been rotated to .log.1" >&2
    exit 1
fi
if [[ -f "$TESTLOG" ]]; then
    echo "FAIL: original 11MB .log file should be gone after rotation" >&2
    exit 1
fi
SIZE1=$(wc -c < "$TESTLOG.1" | tr -d ' ')
if [[ "$SIZE1" -lt 11000000 ]]; then
    echo "FAIL: .log.1 should hold the 11MB content; got $SIZE1 bytes" >&2
    exit 1
fi
echo "PASS: 11MB file rotated to .log.1"

# Test 3: second rotation — .log.1 should be OVERWRITTEN, not promoted to .log.2.
dd if=/dev/zero of="$TESTLOG" bs=1024 count=11264 status=none
rotate_log "$TESTLOG"
if [[ -f "$TESTLOG.2" ]]; then
    echo "FAIL: .log.2 should NOT exist (single-backup retention)" >&2
    exit 1
fi
if [[ ! -f "$TESTLOG.1" ]]; then
    echo "FAIL: .log.1 should still exist after second rotation" >&2
    exit 1
fi
echo "PASS: second rotation overwrote .log.1 (no .log.2)"

# Test 4: rotate_log on a non-existent file — should be a no-op (return 0).
rm -f "$TESTLOG" "$TESTLOG.1"
rotate_log "$TESTLOG" || { echo "FAIL: rotate_log should return 0 for missing file" >&2; exit 1; }
echo "PASS: rotate_log no-op for missing file"

# Test 5: rotate_log on an empty file — should NOT rotate (size = 0 < 10MB).
touch "$TESTLOG"
rotate_log "$TESTLOG"
if [[ -f "$TESTLOG.1" ]]; then
    echo "FAIL: empty file should NOT be rotated" >&2
    exit 1
fi
echo "PASS: empty file not rotated"

echo ""
echo "All rotate_log tests passed."

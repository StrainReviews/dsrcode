#!/bin/bash
# Phase 9 live-daemon verification harness (v4.2.1 hotfix).
#
# Validates:
#   T1 dsrcode binary reports version 4.2.1
#   T2 /health returns HTTP 200
#   T3 SourceClaude UUID session survives >=150s stale-check sweep
#      (0 "removing stale session" lines emitted for the injected UUID)
#
# Preconditions:
#   - Daemon must already be running (start.sh launches it).
#   - `dsrcode` must be on PATH (or invoke via `DSRCODE_BIN=./dsrcode bash scripts/phase-09/verify.sh`).
#   - ~/.claude/dsrcode.log must exist (auto-created on first daemon launch).
#
# Manual follow-up (not automated — wall-clock >2min MCP-heavy reproduction):
#   Start Claude Code, initiate an MCP-heavy flow with >2min silence between hooks,
#   tail ~/.claude/dsrcode.log, assert no "removing stale session" lines for the
#   UUID session across 5+ minutes of activity.
#
# Usage: bash scripts/phase-09/verify.sh
# Exits 0 on all-pass, non-zero on any assertion failure.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${HOME}/.claude/dsrcode.log"
DAEMON_URL="http://127.0.0.1:19460"
DSRCODE_BIN="${DSRCODE_BIN:-dsrcode}"

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1" >&2; exit 1; }

if [[ ! -f "$LOG_FILE" ]]; then
    fail "log file $LOG_FILE not found — daemon not running?"
fi
LOG_OFFSET=$(wc -c < "$LOG_FILE")

CLEANUP_IDS=()
CLEANUP_DONE=false
register_cleanup() {
    CLEANUP_IDS+=("$1")
}
cleanup() {
    local rc=$?
    if $CLEANUP_DONE; then
        return $rc
    fi
    CLEANUP_DONE=true
    set +e
    for id in "${CLEANUP_IDS[@]+"${CLEANUP_IDS[@]}"}"; do
        curl -sS -X POST \
            -H "Content-Type: application/json" \
            -d "{\"session_id\":\"$id\",\"reason\":\"verify-09-sh-cleanup\"}" \
            "${DAEMON_URL}/hooks/session-end" \
            >/dev/null 2>&1 || true
    done
    return $rc
}
trap cleanup EXIT

log_tail() {
    dd if="$LOG_FILE" bs=1 skip="$LOG_OFFSET" 2>/dev/null
}

# --- T1: Binary version ---
if ! command -v "$DSRCODE_BIN" >/dev/null 2>&1; then
    fail "T1 dsrcode binary not on PATH (override with DSRCODE_BIN=./dsrcode)"
fi
VERSION_OUT=$("$DSRCODE_BIN" --version 2>&1 || true)
if echo "$VERSION_OUT" | grep -q "4.2.1"; then
    pass "T1 dsrcode --version contains 4.2.1"
else
    fail "T1 dsrcode --version lacks 4.2.1 — got: $VERSION_OUT"
fi

# --- T2: /health ---
if curl -fsS -o /dev/null "${DAEMON_URL}/health"; then
    pass "T2 /health returned HTTP 200"
else
    fail "T2 /health did not return 200 (daemon not running or bind changed)"
fi

# --- T3: SourceClaude guard holds across >=150s stale-check sweep ---
# Inject a UUID-shaped session_id so sourceFromID returns SourceClaude, then
# send no further hooks for 150s. The Phase-9 D-01 guard must skip the
# PID-liveness branch entirely; zero "removing stale session" lines should
# appear for this UUID during the sweep window.
UUID_ID="verify09-$(date +%s)-abc-def-1234-5678-90ab-cdef"
register_cleanup "$UUID_ID"
curl -fsS -o /dev/null -X POST \
    -H "Content-Type: application/json" \
    -d "{\"session_id\":\"$UUID_ID\",\"tool_name\":\"Edit\",\"cwd\":\"/tmp/verify09\"}" \
    "${DAEMON_URL}/hooks/pre-tool-use"

echo "T3 waiting 150s for stale-check ticker to sweep (mirrors incident elapsed)..."
sleep 150

T3_REMOVE_COUNT=$(log_tail | grep -c "\"removing stale session\".*\"$UUID_ID\"" || true)
if [[ "$T3_REMOVE_COUNT" -eq 0 ]]; then
    pass "T3 SourceClaude UUID session survived PID-liveness sweep (0 removals observed)"
else
    fail "T3 SourceClaude UUID session was removed $T3_REMOVE_COUNT time(s) — v4.2.1 guard regression"
fi

echo ""
echo "All Phase 9 verify tests passed (T1..T3)."

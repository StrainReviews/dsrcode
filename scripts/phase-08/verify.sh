#!/bin/bash
# Phase 8 live-daemon verification harness.
#
# Validates:
#   T1 dsrcode binary reports version 4.2.0
#   T2 /health returns HTTP 200
#   T3 Burst 10 distinct signals -> <=3 coalesced SetActivity calls observed in log
#   T4 Burst 10 identical signals -> >=9 "content_hash" skip events observed in log
#   T5 Duplicate POST /hooks/pre-tool-use within 500ms -> exactly 1 "hook deduped" log
#   T6 Periodic 60s "coalescer status" INFO summary appears once counters are non-zero
#
# Preconditions: Daemon must already be running (start.sh launches it).
# Usage: bash scripts/phase-08/verify.sh
# Exits 0 on all-pass, non-zero on any assertion failure.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${HOME}/.claude/dsrcode.log"
DAEMON_URL="http://127.0.0.1:19460"

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1" >&2; exit 1; }

# Record log offset at start so T3..T6 only look at lines written during this run.
if [[ ! -f "$LOG_FILE" ]]; then
    fail "log file $LOG_FILE not found — daemon not running?"
fi
LOG_OFFSET=$(wc -c < "$LOG_FILE")

log_tail() {
    # Print log bytes written after LOG_OFFSET.
    dd if="$LOG_FILE" bs=1 skip="$LOG_OFFSET" 2>/dev/null
}

# --- T1: Binary version ---
if ! command -v dsrcode >/dev/null 2>&1; then
    fail "T1 dsrcode binary not on PATH"
fi
VERSION_OUT=$(dsrcode --version 2>&1 || true)
if echo "$VERSION_OUT" | grep -q "4.2.0"; then
    pass "T1 dsrcode --version contains 4.2.0"
else
    fail "T1 dsrcode --version lacks 4.2.0 — got: $VERSION_OUT"
fi

# --- T2: /health ---
if curl -fsS -o /dev/null "${DAEMON_URL}/health"; then
    pass "T2 /health returned HTTP 200"
else
    fail "T2 /health did not return 200 (daemon not running or bind changed)"
fi

# --- T3: Burst coalescing (10 distinct session IDs -> <=3 SetActivity calls) ---
for i in $(seq 1 10); do
    curl -fsS -o /dev/null -X POST \
        -H "Content-Type: application/json" \
        -d "{\"session_id\":\"t3-s${i}\",\"tool_name\":\"Edit\",\"cwd\":\"/tmp/t3\"}" \
        "${DAEMON_URL}/hooks/pre-tool-use"
done
# Wait for debounce + first coalesced flush + subsequent bucket refills.
sleep 10

T3_COUNT=$(log_tail | grep -c '"presence updated"' || true)
if [[ "$T3_COUNT" -le 3 ]]; then
    pass "T3 10 distinct signals produced $T3_COUNT SetActivity calls (<=3 coalesced)"
else
    fail "T3 expected <=3 coalesced SetActivity calls, got $T3_COUNT"
fi

# --- T4: Content-hash skip (10 identical signals -> >=9 hash-skips) ---
IDENT_BODY='{"session_id":"t4-s1","tool_name":"Edit","cwd":"/tmp/t4"}'
for i in $(seq 1 10); do
    curl -fsS -o /dev/null -X POST \
        -H "Content-Type: application/json" \
        -d "$IDENT_BODY" \
        "${DAEMON_URL}/hooks/pre-tool-use" || true
    # Yield tiny spacing so dedup TTL passes between each
    sleep 0.6
done
sleep 2

T4_COUNT=$(log_tail | grep -c '"presence update skipped".*"reason":"content_hash"' || true)
if [[ "$T4_COUNT" -ge 9 ]]; then
    pass "T4 $T4_COUNT content_hash skips observed (>=9 expected from 10 identical signals)"
else
    fail "T4 expected >=9 content_hash skip events, got $T4_COUNT"
fi

# --- T5: Hook dedup (2 identical POSTs within 500ms -> exactly 1 dedup log) ---
DEDUP_BODY='{"session_id":"t5-s1","tool_name":"Grep","cwd":"/tmp/t5"}'
curl -fsS -o /dev/null -X POST -H "Content-Type: application/json" -d "$DEDUP_BODY" "${DAEMON_URL}/hooks/pre-tool-use"
curl -fsS -o /dev/null -X POST -H "Content-Type: application/json" -d "$DEDUP_BODY" "${DAEMON_URL}/hooks/pre-tool-use"
sleep 1

T5_COUNT=$(log_tail | grep -c '"hook deduped"' || true)
if [[ "$T5_COUNT" -eq 1 ]]; then
    pass "T5 exactly 1 hook dedup observed for 2 identical POSTs within 500ms"
else
    fail "T5 expected exactly 1 hook dedup, got $T5_COUNT"
fi

# --- T6: 60s summary log ---
echo "T6 waiting 65s for coalescer-status summary..."
sleep 65

T6_COUNT=$(log_tail | grep -c '"coalescer status"' || true)
if [[ "$T6_COUNT" -ge 1 ]]; then
    pass "T6 coalescer status summary appeared $T6_COUNT time(s)"
else
    fail "T6 no coalescer status summary observed after 65s wait"
fi

echo ""
echo "All Phase 8 verify tests passed (T1..T6)."

#!/bin/bash
# Forward Claude Code hook data to the Go HTTP server.
# Reads JSON from stdin (Claude Code hook format), POSTs to localhost.
# Fire-and-forget: always exits 0 so hooks never block Claude Code.

HOOK_TYPE="${1:-unknown}"
PORT=19460

# Read stdin (Claude Code passes hook JSON via stdin)
input=$(cat)

# Forward to Go server (fire-and-forget, 1s timeout)
echo "$input" | curl -s -X POST \
  -H "Content-Type: application/json" \
  -d @- \
  --max-time 1 \
  "http://127.0.0.1:${PORT}/hooks/${HOOK_TYPE}" > /dev/null 2>&1 || true

# Pass through stdin to stdout (required by Claude Code)
echo "$input"

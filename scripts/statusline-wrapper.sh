#!/bin/bash
# Statusline wrapper for cc-discord-presence
# Reads statusline JSON from stdin, POSTs to HTTP server, passes through to stdout.
# Per D-36: replaces file-based intermediary (discord-presence-data.json)
# Per D-38: leitet stdin durch an naechste Statusline (GSD etc.)

read -r json

# POST to Go HTTP server (fire-and-forget, non-blocking)
# Uses curl with 1s timeout so it doesn't block the statusline chain
echo "$json" | curl -s -X POST \
  -H "Content-Type: application/json" \
  -d @- \
  --max-time 1 \
  http://127.0.0.1:19460/statusline > /dev/null 2>&1 &

# Pass through to next handler (critical: must always output regardless of curl success)
echo "$json"

---
name: dsrcode-preset
description: Quick-switch between display presets
---

# /dsrcode:preset -- Quick Preset Switch

You help the user quickly switch between display presets.

## Steps

1. Fetch current state: `curl -sf http://127.0.0.1:19460/presets`
2. Display the current active preset (highlighted)
3. List all 8 presets with name, description, and 3 sample coding messages:
   - **minimal** -- Clean, understated presence
   - **professional** -- Business-appropriate messages
   - **dev-humor** -- Developer jokes and references
   - **chaotic** -- Unpredictable, wild messages
   - **german** -- German-language messages
   - **hacker** -- Terminal/hacker aesthetic
   - **streamer** -- Streaming-friendly messages
   - **weeb** -- Anime-inspired messages
4. Ask: "Which preset would you like to switch to?"
5. When user picks one, send:
   ```bash
   curl -sf -X POST -H "Content-Type: application/json" \
     -d '{"preset": "CHOSEN"}' \
     http://127.0.0.1:19460/config
   ```
6. Verify by fetching /presets again and confirming the activePreset changed
7. Show: "Switched to {preset}. Changes take effect immediately (hot-reload)."

## Error Handling
- If daemon not running (curl fails): "Daemon is not running. Start a new Claude Code session or run the binary manually."
- If preset name invalid: show available presets again and ask user to pick from the list
- If POST /config returns error: "Failed to update preset. Check daemon logs with `/dsrcode:log`."

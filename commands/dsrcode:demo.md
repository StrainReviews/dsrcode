# /dsrcode:demo -- Preview Mode for Screenshots

You help the user preview how DSR Code Presence looks on Discord for screenshot generation. Uses POST /preview to set a temporary presence.

## Steps

1. Check daemon is running: `curl -sf http://127.0.0.1:19460/health 2>/dev/null`
   - Not running: "Daemon must be running. Run `/clear` to start it."

2. Ask: "Which preset would you like to preview?"
   - Fetch presets: `curl -sf http://127.0.0.1:19460/presets`
   - Show the 8 options with sample messages

3. Ask: "Which display detail level? (minimal/standard/verbose/private)"
   - **minimal**: Project name only
   - **standard**: File names and short commands
   - **verbose**: Full paths and commands
   - **private**: All info hidden

4. Set the preview via POST /preview:
   ```bash
   curl -sf -X POST -H "Content-Type: application/json" \
     -d '{
       "preset": "CHOSEN",
       "displayDetail": "LEVEL",
       "duration": 120,
       "details": "Working on my-project",
       "state": "Sonnet 4.6 | 150K tokens | $1.50 | 2h 15m"
     }' \
     http://127.0.0.1:19460/preview
   ```

5. Show: "Preview is now active on Discord for 120 seconds. Switch to Discord and take your screenshot!"

6. Ask: "Ready to try another preset, or done?"
   - Another: go back to step 2
   - Done: "Preview will auto-expire. Normal presence will resume."

## Quick Preset Tour
If user wants to screenshot all presets:
1. Loop through each of the 8 presets
2. For each: POST /preview with the preset, wait for user to confirm screenshot taken
3. After all 8: "All presets captured!"

## Tips
- Screenshots should show the Discord profile popup with the Rich Presence
- For README screenshots, use each preset once with "standard" display detail
- The preview shows exactly what other users would see on your Discord profile
- Preview duration can be adjusted (5-300 seconds), default is 120 seconds
- To cancel a preview early, POST a new preview or wait for it to expire

---
name: demo
description: "Preview Discord presence for screenshots and demos"
disable-model-invocation: true
---

# /dsrcode:demo -- Discord Presence Demo & Screenshot Tool

You help the user preview how DSR Code Presence looks on Discord. This skill has 4 modes for different screenshot and demo scenarios.

## Entry Point

1. Verify daemon is running:
   ```bash
   curl -sf http://127.0.0.1:19460/health 2>/dev/null
   ```
   - If it fails: "Daemon is not running. Start it with `/clear` or run the start script first."
   - If it succeeds: continue to menu.

2. Present the mode menu:

   **Which demo mode?**
   1. **Quick Preview** -- Single preset, one screenshot
   2. **Preset Tour** -- Walk through all 8 presets sequentially
   3. **Multi-Session Preview** -- Simulate multiple projects open
   4. **Message Rotation** -- See how messages rotate over time

---

## Mode 1: Quick Preview

Single preset preview for taking one clean screenshot.

1. Fetch available presets:
   ```bash
   curl -sf http://127.0.0.1:19460/presets
   ```
   Show the preset names to the user (minimal, professional, dev-humor, chaotic, german, hacker, streamer, weeb).

2. Ask: "Which preset?" and "Which display detail level? (minimal / standard / verbose / private)"

3. Calculate a start timestamp 2 hours in the past:
   ```bash
   START_TS=$(( $(date +%s) - 7200 ))
   ```

4. Set the preview via POST /preview with realistic demo data:
   ```bash
   curl -sf -X POST -H "Content-Type: application/json" \
     -d '{
       "preset": "CHOSEN_PRESET",
       "displayDetail": "CHOSEN_LEVEL",
       "duration": 120,
       "details": "Working on my-saas-app",
       "state": "Opus 4.6 | 250K tokens | $2.50 | 2h 15m",
       "smallImage": "coding",
       "smallText": "Editing auth.ts",
       "largeText": "my-saas-app (feature/auth)",
       "startTimestamp": START_TS
     }' \
     http://127.0.0.1:19460/preview
   ```
   Replace CHOSEN_PRESET and CHOSEN_LEVEL with user choices. Replace START_TS with the calculated epoch value.

5. Tell the user: "Preview active for 120 seconds. Switch to Discord and take your screenshot."

6. Ask: "Another preset, or done?"
   - Another: go back to step 2
   - Done: "Preview will auto-expire. Normal presence resumes after that."

---

## Mode 2: Preset Tour

Walk through all 8 presets sequentially. Claude controls the loop -- user confirms each screenshot.

1. Fetch available presets:
   ```bash
   curl -sf http://127.0.0.1:19460/presets
   ```

2. For each preset in order (minimal, professional, dev-humor, chaotic, german, hacker, streamer, weeb):

   a. Calculate start timestamp:
      ```bash
      START_TS=$(( $(date +%s) - 7200 ))
      ```

   b. POST /preview with the current preset and realistic data:
      ```bash
      curl -sf -X POST -H "Content-Type: application/json" \
        -d '{
          "preset": "CURRENT_PRESET",
          "displayDetail": "standard",
          "duration": 120,
          "details": "Working on my-saas-app",
          "state": "Opus 4.6 | 250K tokens | $2.50 | 2h 15m",
          "smallImage": "coding",
          "smallText": "Editing auth.ts",
          "largeText": "my-saas-app (feature/auth)",
          "startTimestamp": START_TS
        }' \
        http://127.0.0.1:19460/preview
      ```
      Replace CURRENT_PRESET with the preset name. Replace START_TS with the epoch.

   c. Tell the user: "Now showing: **CURRENT_PRESET**. Switch to Discord to see it."

   d. Ask: "Screenshot taken? (yes / skip / stop)"
      - **yes** or **skip**: continue to next preset
      - **stop**: exit the loop early

3. After all 8 presets (or stop): "All presets captured!" or "Tour ended at PRESET."

---

## Mode 3: Multi-Session Preview

Simulate multiple concurrent projects for a multi-session Discord screenshot.

1. Ask: "How many projects? (2-5)"

2. POST /preview with fakeSessions. Example for 3 projects:
   ```bash
   START_TS=$(( $(date +%s) - 7200 ))
   curl -sf -X POST -H "Content-Type: application/json" \
     -d '{
       "preset": "minimal",
       "displayDetail": "standard",
       "duration": 120,
       "sessionCount": 3,
       "startTimestamp": '"$START_TS"',
       "fakeSessions": [
         {
           "projectName": "my-saas-app",
           "model": "Opus 4.6",
           "branch": "feature/auth",
           "totalTokens": 250000,
           "totalCost": 2.50,
           "activity": "coding",
           "status": "active"
         },
         {
           "projectName": "api-gateway",
           "model": "Sonnet 4.6",
           "branch": "main",
           "totalTokens": 180000,
           "totalCost": 1.20,
           "activity": "terminal",
           "status": "active"
         },
         {
           "projectName": "docs-site",
           "model": "Haiku 4.5",
           "branch": "main",
           "totalTokens": 50000,
           "totalCost": 0.30,
           "activity": "reading",
           "status": "idle"
         }
       ]
     }' \
     http://127.0.0.1:19460/preview
   ```
   Adjust project count and data based on user's choice. Use realistic project names and data.

3. Tell the user: "Multi-session preview active. Discord should show SESSION_COUNT projects."

4. Ask: "Adjust projects, change preset, or done?"
   - Adjust: ask for changes, re-POST with new data
   - Change preset: re-POST with different preset
   - Done: "Preview will auto-expire."

---

## Mode 4: Message Rotation

See how status messages rotate over time for a given preset and activity.

1. Ask: "Which preset?" and "Which activity? (coding / terminal / searching / reading / thinking)"

2. Fetch rotation messages via GET /preview/messages:
   ```bash
   curl -sf "http://127.0.0.1:19460/preview/messages?preset=CHOSEN_PRESET&activity=CHOSEN_ACTIVITY&count=10&detail=standard"
   ```
   Replace CHOSEN_PRESET and CHOSEN_ACTIVITY with user's choices.

3. Display the returned messages in a formatted list. Each message represents what Discord would show in a different 5-minute time window:
   - Show the `details` and `state` fields from each returned Activity object
   - Show the `smallImage` icon for each entry
   - Number them 1-N to show the rotation sequence

4. Ask: "Try another preset/activity combo, or done?"
   - Another: go back to step 1
   - Done: "Message rotation preview complete."

---

## General Guidelines

- All curl commands target `http://127.0.0.1:19460`
- `startTimestamp` is a Unix epoch -- calculate as current time minus desired session duration
- Do NOT include any "Preview Mode" text in any payload. Screenshots should look authentic.
- Use realistic project names and data: "my-saas-app", "api-gateway", "docs-site", "mobile-app", "infra-config"
- Realistic models: "Opus 4.6", "Sonnet 4.6", "Haiku 4.5"
- Realistic token counts: 50K-500K range
- Realistic costs: $0.30-$5.00 range
- Preview duration can be adjusted (5-300 seconds), default is 120
- To cancel a preview early, POST a new preview or wait for it to expire

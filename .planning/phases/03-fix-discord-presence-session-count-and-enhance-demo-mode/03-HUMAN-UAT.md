---
status: partial
phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode
source: [16-VERIFICATION.md]
started: 2026-04-06T13:00:00Z
updated: 2026-04-06T13:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Discord session count in production
expected: Build/run daemon, open 1 Claude Code window → GET /sessions shows 1 session with "source" field → Discord shows "1 Projekt" not "Aktiv in 2 Repos"
result: [pending]

### 2. Demo skill 4-mode interaction
expected: Run /dsrcode:demo → 4-mode menu appears → all modes functional → no "Preview Mode" text in Discord → skill-controlled preset tour loop
result: [pending]

### 3. Binary version output
expected: Build binary → run --version → shows "3.1.0"
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps

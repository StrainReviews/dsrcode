---
phase: 05-binary-distribution
plan: 04
subsystem: scripts
tags: [start-scripts, binary-distribution, download, sha256, migration]
dependency_graph:
  requires: [05-01]
  provides: [start-sh-download, start-ps1-download, binary-migration]
  affects: [scripts/start.sh, scripts/start.ps1]
tech_stack:
  added: [sha256-checksum-verification, lock-file-protection, archive-extraction]
  patterns: [download-first-with-build-fallback, atomic-download-via-tempdir, lock-file-concurrency]
key_files:
  created: []
  modified: [scripts/start.sh, scripts/start.ps1]
decisions:
  - "CLAUDE_PLUGIN_DATA as primary binary storage with $HOME/.claude/plugins/data/dsrcode fallback"
  - "Archive-based download (tar.gz/zip) with SHA256 checksums.txt verification"
  - "Lock-file uses flock on Unix with PID-based fallback for portability"
  - "Old binary migration copies then deletes (not moves) for safety"
  - "set -e scoped after binary acquisition to allow download/build failure flow control"
metrics:
  duration: 1269s
  completed: 2026-04-09
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
requirements_completed: [DIST-18, DIST-19, DIST-20, DIST-21, DIST-22, DIST-23, DIST-24, DIST-26, DIST-27, DIST-28, DIST-37, DIST-40, DIST-43]
---

# Phase 5 Plan 4: Start Script Rewrite Summary

Download-first binary acquisition via GitHub Releases with SHA256 checksum verification, lock-file concurrency protection, old binary migration, and CLAUDE_PLUGIN_DATA persistent storage for both bash and PowerShell scripts.

## What Changed

### Task 1: Complete rewrite of start.sh (77b0ab9)

Replaced the build-only start.sh with a download-first strategy. The new script:

1. **Configuration:** Uses `CLAUDE_PLUGIN_DATA` for binary storage with `$HOME/.claude/plugins/data/dsrcode` fallback. All runtime files renamed to `dsrcode-*` (pid, log, sessions, refcount).

2. **Download with SHA256:** Downloads `dsrcode_X.Y.Z_os_arch.tar.gz` archive from GitHub Releases, verifies SHA256 checksum against `dsrcode_X.Y.Z_checksums.txt`, extracts `dsrcode` binary. Size check rejects HTML error pages.

3. **Lock-file protection:** Uses `flock` when available (Linux), falls back to PID-based lock file (macOS/Windows Git Bash). 30-second timeout with stale lock detection.

4. **Build-from-source fallback:** Checks `CLAUDE_PLUGIN_ROOT`, `~/Projects/cc-discord-presence`, and `~/Projects/dsrcode` for Go source. Uses lowercase `-X main.version` ldflags per DIST-06.

5. **Old binary migration (DIST-37):** Detects `~/.claude/bin/cc-discord-presence-*`, kills running daemon (checking both old and new PID paths), copies to new location, deletes old binary.

6. **Version check:** Skips update for `dev` or `unknown` versions per DIST-22. Parses `dsrcode X.Y.Z (commit, date)` output via `awk '{print $2}'`.

7. **Preserved features:** Session tracking (refcount on Windows, PID files on Unix), hooks autopatch (Agent matcher + SubagentStop), german preset migration, statusline-wrapper auto-update (DIST-28), health check on port 19460 (DIST-40), first-run hint.

8. **Error handling:** `set -e` only after binary is secured. Download/build failures use return codes, not exit. Proxy hint (`HTTP_PROXY/HTTPS_PROXY`) on download failure per DIST-27.

### Task 2: Complete rewrite of start.ps1 (591d851)

Feature parity with start.sh for Windows PowerShell:

1. **Configuration:** Same CLAUDE_PLUGIN_DATA pattern, same dsrcode-* runtime file names.

2. **Download with SHA256:** Uses `Invoke-WebRequest -UseBasicParsing` for download, `Get-FileHash -Algorithm SHA256` for verification (both built-in, no external deps per DIST-24), `Expand-Archive` for zip extraction.

3. **Lock-file protection:** PID-based lock file with stale process detection and 30-second timeout.

4. **Build-from-source fallback:** Same source directory search as bash, Go build with ldflags.

5. **Old binary migration:** Detects `cc-discord-presence-windows-amd64.exe`, kills daemon, copies to new location.

6. **Version check:** Regex-based parsing of `--version` output. Skips `dev`/`unknown`.

7. **Daemon start:** `Start-Process -WindowStyle Hidden` with PID capture.

8. **Health check:** `Invoke-WebRequest` to `http://127.0.0.1:19460/health` with 5-second polling (50 iterations x 100ms).

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Archive-based download (not raw binary) | GoReleaser produces archives with checksums. Archives also include LICENSE/README. |
| SHA256 checksums.txt from same release | Same trust boundary as binary (GitHub Releases over HTTPS). No separate signing per DIST-12. |
| Lock-file with flock + PID fallback | flock is atomic but not available on all platforms. PID-based fallback is universal. |
| Copy-then-delete for migration | Safer than move -- if copy fails, old binary remains functional. |
| ErrorActionPreference = Continue | PowerShell must not abort on download failure -- fallback flow depends on it. |

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

### start.sh
- Syntax check: `bash -n` passes (RC=0)
- CLAUDE_PLUGIN_DATA: 2 references
- SHA256: 5 references
- Lock-file: 16 references
- cc-discord-presence-*: 1 reference (migration section only)
- dsrcode.pid: present
- dsrcode.log: present
- dsrcode-sessions: present
- StrainReviews/dsrcode: present
- VERSION="v4.0.0": present
- Port 19460: 4 references
- set -e: line 377 (after binary acquisition, not at top)
- No tsanva references
- No build.sh references

### start.ps1
- CLAUDE_PLUGIN_DATA: 2 references
- Get-FileHash -Algorithm SHA256: 2 references
- Expand-Archive: 2 references
- Invoke-WebRequest: 3 references
- Lock-file: 28 references
- cc-discord-presence-windows-amd64.exe: migration section only
- dsrcode.pid, dsrcode.log, dsrcode.refcount: all present
- StrainReviews/dsrcode: present
- $Version = "v4.0.0": present
- Port 19460: 3 references
- dev/unknown skip: present
- Proxy hint: 2 references
- No tsanva references
- No build.sh references

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 77b0ab9 | start.sh rewrite with download-first, SHA256, lock-file, migration |
| 2 | 591d851 | start.ps1 rewrite with feature parity to start.sh |

## Self-Check: PASSED

- scripts/start.sh: FOUND
- scripts/start.ps1: FOUND
- 05-04-SUMMARY.md: FOUND
- commit 77b0ab9: FOUND
- commit 591d851: FOUND

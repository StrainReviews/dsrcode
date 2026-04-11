# Phase 6.1 Handoff Checklist

**Purpose:** Manually rename `C:\Users\ktown\Projects\cc-discord-presence` → `dsrcode`, migrate Claude Code memory directory, reinstall plugin on v4.1.0.

**When to use:** After Plans 01-04 are committed, before Plan 05 runs in a new Claude Code session.

---

## ⚠️ CRITICAL: Why the Handoff Pause Exists

Windows does not permit renaming a directory that is the current working directory of ANY running process (see Stack Overflow #41365318). The running Claude Code session has its cwd inside the project directory, so the rename MUST happen from an external shell after closing the Claude Code session.

---

## Section 1: Prerequisites

**Before running handoff.ps1, verify ALL of the following:**

### 1.1 Plan 01 CI Success
- [ ] https://github.com/StrainReviews/dsrcode/actions — latest "Release" workflow for tag `v4.1.0` shows ✅ green
- [ ] https://github.com/StrainReviews/dsrcode/releases/tag/v4.1.0 loads (not 404)
- [ ] Release page shows exactly 6 assets:
  - [ ] `dsrcode_4.1.0_darwin_amd64.tar.gz`
  - [ ] `dsrcode_4.1.0_darwin_arm64.tar.gz`
  - [ ] `dsrcode_4.1.0_linux_amd64.tar.gz`
  - [ ] `dsrcode_4.1.0_linux_arm64.tar.gz`
  - [ ] `dsrcode_4.1.0_windows_amd64.zip`
  - [ ] `dsrcode_4.1.0_checksums.txt`

### 1.2 Close IDE and Editor Windows
- [ ] VS Code windows for this project CLOSED
- [ ] JetBrains IDE (GoLand, WebStorm, etc.) CLOSED
- [ ] Cursor windows CLOSED
- [ ] Notepad++ / Sublime / other editors with files from this project CLOSED

### 1.3 Close Shells
- [ ] All Git Bash windows with cwd in project CLOSED
- [ ] All PowerShell windows with cwd in project CLOSED
- [ ] All Windows Terminal tabs with cwd in project CLOSED
- [ ] **EXCEPT** the external PowerShell you'll use to run `handoff.ps1` (should NOT be inside the project dir)

### 1.4 Close Running Claude Code Session
- [ ] Exit current Claude Code session with Ctrl+D or `/exit`
- [ ] Confirm no `claude.exe` process remains (check Task Manager)

### 1.5 OneDrive Sync (only if applicable)
- [ ] If project is inside OneDrive folder (NOT our case for `C:\Users\ktown\Projects\`): pause OneDrive sync

### 1.6 Uncommitted Changes
- [ ] `git status` clean (no uncommitted changes from Plans 01-04)
- [ ] `git log --oneline -5` shows the Plan 01-04 commits at HEAD

---

## Section 2: Execution Steps

### Step 2.1: Open External PowerShell
Open a NEW PowerShell window. **Start it from a directory that is NOT inside the project.** Example:

```powershell
cd C:\Users\ktown
```

### Step 2.2: Run Prereq Check
```powershell
C:\Users\ktown\Projects\cc-discord-presence\scripts\phase-06.1\prereq-check.ps1
```

Interpret the exit code:
- **Exit 0:** All checks passed. Proceed to Step 2.3.
- **Exit 1:** Warnings (e.g., Handle.exe not installed, Go process running). Review the warnings. If they're expected, proceed.
- **Exit 2:** BLOCKERS. DO NOT PROCEED. Resolve the blockers first. Common fixes:
  - Daemon still running: `Stop-Process -Id 55564 -Force`
  - IDE handle on files: Close the IDE window
  - Run the prereq check again until exit ≤ 1.

### Step 2.3: Run Handoff Script
Optionally test with dry-run first:
```powershell
C:\Users\ktown\Projects\cc-discord-presence\scripts\phase-06.1\handoff.ps1 -DryRun
```

Review the dry-run output. When satisfied, run live:
```powershell
C:\Users\ktown\Projects\cc-discord-presence\scripts\phase-06.1\handoff.ps1
```

**Watch the output carefully.** Each step prints its status. If any step FAILS (red ✗), see Section 3 (Rollback).

### Step 2.4: Open New Claude Code Session
After handoff.ps1 completes successfully:

```powershell
cd C:\Users\ktown\Projects\dsrcode
claude
```

The new session will auto-load Phase 6.1 memory and the v4.1.0 plugin.

### Step 2.5: Run Verification
In the NEW Claude Code session, either:
- Ask: "Run Phase 6.1 Plan 05" (which runs verify.ps1 + updates docs), OR
- Manually run: `.\scripts\phase-06.1\verify.ps1`

---

## Section 3: Rollback

### 3.1 Rollback after Step 3 (project dir renamed but rest failed)
```powershell
Rename-Item -Path 'C:\Users\ktown\Projects\dsrcode' -NewName 'cc-discord-presence'
```

### 3.2 Rollback after Step 4 (memory dir also renamed)
```powershell
Rename-Item -Path "$env:USERPROFILE\.claude\projects\C--Users-ktown-Projects-dsrcode" -NewName 'C--Users-ktown-Projects-cc-discord-presence'
```

### 3.3 Rollback after Step 6 (cache cleaned — cannot unroll)
The cache is regenerated on next plugin install, so no true rollback needed. Continue forward with reinstall.

### 3.4 Rollback after Step 7 (plugin uninstalled)
```powershell
claude plugin install dsrcode@dsrcode
```
Re-install the plugin. This is also Step 8 of handoff.ps1 — the script already handles this if it reached step 8.

### 3.5 Full rollback (v4.1.0 release broken)
If the v4.1.0 release is broken (smoke tests fail), execute D-16:
```bash
gh release delete v4.1.0 --yes
git tag -d v4.1.0
git push origin :refs/tags/v4.1.0
# Fix the issue, then redo Plan 01
```

---

## Section 4: Troubleshooting

### "Rename-Item: The process cannot access the file because it is being used by another process"
The rename failed because a process holds a handle. Find and kill:

```powershell
# Method A: Handle.exe (if installed)
handle.exe -a -u 'C:\Users\ktown\Projects\cc-discord-presence'

# Method B: Search by CommandLine
Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like '*cc-discord-presence*' }
```

Kill each PID reported, then re-run handoff.ps1.

### "claude: command not found"
Claude CLI not in PATH. Full path:
```powershell
C:\Users\ktown\.local\bin\claude.cmd plugin install dsrcode@dsrcode
```

### "git pull failed" in Step 5
Marketplace clone may have local modifications or be in detached HEAD:
```powershell
cd "$env:USERPROFILE\.claude\plugins\marketplaces\dsrcode"
git status
git stash   # if dirty
git checkout main
git pull origin main
```

### "Plugin install failed — plugin not found in marketplace"
The marketplace cache is stale. Force refresh:
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.claude\plugins\marketplaces\dsrcode"
# Then re-add the marketplace via Claude Code /plugin interface
```

### "New Claude Code session doesn't see memory"
Verify memory dir was renamed:
```powershell
Test-Path "$env:USERPROFILE\.claude\projects\C--Users-ktown-Projects-dsrcode"
```

If false, manually run:
```powershell
Rename-Item `
  -Path "$env:USERPROFILE\.claude\projects\C--Users-ktown-Projects-cc-discord-presence" `
  -NewName 'C--Users-ktown-Projects-dsrcode'
```

### "T1-T8 smoke tests fail"
Re-run `.\scripts\phase-06.1\verify.ps1` with verbose mode. Common causes:
- T1 fails: Binary not v4.1.0 → check start.sh DIST-23 download from CLAUDE_PLUGIN_DATA
- T2 fails: Daemon not running → check plugin SessionStart hook fired
- T4 fails: settings.local.json missing hooks → start.sh auto-patch failed → manual re-run of start.sh
- T7 fails: Auto-exit not firing → Phase 6 D-05 grace period config check

---

**Last updated:** 2026-04-10 (Phase 6.1 Plan 03)
**Script references:**
- `scripts/phase-06.1/prereq-check.ps1` (Plan 02)
- `scripts/phase-06.1/handoff.ps1` (this plan)
- `scripts/phase-06.1/verify.ps1` (Plan 04)

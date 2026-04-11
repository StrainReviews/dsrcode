---
phase: quick-260411-iyf
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - scripts/start.sh
autonomous: true
requirements:
  - QUICK-260411-IYF
must_haves:
  truths:
    - "Windows dsrcode daemon writes slog output to ~/.claude/dsrcode.log after SessionStart"
    - "Windows dsrcode daemon stderr is captured to ~/.claude/dsrcode.log.err (not lost to the void)"
    - "Existing -WindowStyle Hidden + -PassThru + PID-capture behavior is preserved"
    - "Unix (nohup) launch path is unchanged"
  artifacts:
    - path: "scripts/start.sh"
      provides: "Windows daemon launch with stdout/stderr redirection"
      contains: "RedirectStandardOutput"
  key_links:
    - from: "scripts/start.sh Windows branch"
      to: "powershell.exe Start-Process"
      via: "-RedirectStandardOutput $WIN_LOG_FILE -RedirectStandardError ${WIN_LOG_FILE}.err"
      pattern: "RedirectStandardOutput.*RedirectStandardError"
---

<objective>
Fix Windows daemon launch in scripts/start.sh so that dsrcode's stdout (slog output) and stderr are captured to log files instead of being lost to the void.

Purpose: Currently the Windows launch branch (lines 568-575) calls `powershell.exe ... Start-Process ... -WindowStyle Hidden -PassThru` with NO stdout/stderr redirection. dsrcode's Go slog writes to stdout, so under Windows all daemon output disappears — making crashes silently invisible. The Unix branch already uses `nohup "$BINARY" > "$LOG_FILE" 2>&1 &`. This plan brings the Windows branch to parity.

Output: scripts/start.sh with Windows Start-Process call updated to redirect stdout → dsrcode.log and stderr → dsrcode.log.err, plus a new `WIN_LOG_FILE` variable derived via `cygpath -w`.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
</execution_context>

<context>
@.planning/STATE.md
@scripts/start.sh
@scripts/start.ps1

# Root cause (locked — do not re-research):
#
# scripts/start.sh lines 568-575 (current):
#
#   if $IS_WINDOWS; then
#       WIN_BINARY=$(cygpath -w "$BINARY" 2>/dev/null || echo "$BINARY")
#       WIN_PID_FILE=$(cygpath -w "$PID_FILE" 2>/dev/null || echo "$PID_FILE")
#
#       powershell.exe -NoProfile -WindowStyle Hidden -Command \
#           '$process = Start-Process -FilePath "'"$WIN_BINARY"'" -WindowStyle Hidden -PassThru; $process.Id | Out-File -FilePath "'"$WIN_PID_FILE"'" -Encoding ASCII -NoNewline' 2>/dev/null
#   else
#       nohup "$BINARY" > "$LOG_FILE" 2>&1 &
#       echo $! > "$PID_FILE"
#   fi
#
# Problem: Start-Process has no -RedirectStandardOutput / -RedirectStandardError.
# dsrcode's Go slog writes to stdout → lost.
#
# Constraint: PowerShell's Start-Process throws "MutuallyExclusiveArguments" if
# -RedirectStandardOutput and -RedirectStandardError point to the same path.
# Fix: stdout → $WIN_LOG_FILE, stderr → "${WIN_LOG_FILE}.err".
#
# Note: $LOG_FILE exists in start.sh (used by the Unix branch). Need to add
# $WIN_LOG_FILE via cygpath -w, mirroring the existing WIN_BINARY / WIN_PID_FILE
# lines at 571-572.
#
# Out of scope: start.ps1 has a latent bug at line 335 (same path for both
# redirect args) — flagged separately, not fixed here.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Redirect Windows daemon stdout/stderr in scripts/start.sh</name>
  <files>scripts/start.sh</files>
  <action>
Edit the Windows launch block at lines 568-575 of scripts/start.sh. Make exactly these changes — no other edits:

1. After the existing `WIN_PID_FILE=$(cygpath -w "$PID_FILE" 2>/dev/null || echo "$PID_FILE")` line (line 572), add a new line converting $LOG_FILE:

   ```bash
   WIN_LOG_FILE=$(cygpath -w "$LOG_FILE" 2>/dev/null || echo "$LOG_FILE")
   ```

2. Update the `powershell.exe -NoProfile -WindowStyle Hidden -Command '...'` invocation so the embedded `Start-Process` call includes both redirect arguments. The final command must:
   - Keep `-FilePath "$WIN_BINARY"` (unchanged)
   - Keep `-WindowStyle Hidden -PassThru` (unchanged)
   - Add `-RedirectStandardOutput "$WIN_LOG_FILE"` → stdout goes to same dsrcode.log as the Unix path
   - Add `-RedirectStandardError "${WIN_LOG_FILE}.err"` → stderr goes to a sibling .err file (PowerShell rejects same-path for both streams with "MutuallyExclusiveArguments")
   - Keep the `$process.Id | Out-File -FilePath "$WIN_PID_FILE" -Encoding ASCII -NoNewline` PID capture (unchanged)
   - Keep the trailing `2>/dev/null` on the bash side (unchanged)

   The resulting block should look like:

   ```bash
   if $IS_WINDOWS; then
       WIN_BINARY=$(cygpath -w "$BINARY" 2>/dev/null || echo "$BINARY")
       WIN_PID_FILE=$(cygpath -w "$PID_FILE" 2>/dev/null || echo "$PID_FILE")
       WIN_LOG_FILE=$(cygpath -w "$LOG_FILE" 2>/dev/null || echo "$LOG_FILE")

       powershell.exe -NoProfile -WindowStyle Hidden -Command \
           '$process = Start-Process -FilePath "'"$WIN_BINARY"'" -WindowStyle Hidden -PassThru -RedirectStandardOutput "'"$WIN_LOG_FILE"'" -RedirectStandardError "'"${WIN_LOG_FILE}.err"'"; $process.Id | Out-File -FilePath "'"$WIN_PID_FILE"'" -Encoding ASCII -NoNewline' 2>/dev/null
   else
       nohup "$BINARY" > "$LOG_FILE" 2>&1 &
       echo $! > "$PID_FILE"
   fi
   ```

Do NOT touch:
- The Unix `else` branch (nohup line)
- start.ps1 (out of scope — its latent same-path-bug is flagged separately for a follow-up quick)
- Any version strings or CHANGELOG (user handles release bump separately)
- Any other file

Preserve the existing bash-quoting idiom (`'"..."'` to interpolate shell variables inside the single-quoted PowerShell command string) — that is how the existing WIN_BINARY/WIN_PID_FILE interpolations already work.
  </action>
  <verify>
    <automated>bash -n scripts/start.sh &amp;&amp; grep -c "RedirectStandardOutput" scripts/start.sh &amp;&amp; grep -c "RedirectStandardError" scripts/start.sh &amp;&amp; grep -c 'WIN_LOG_FILE=$(cygpath' scripts/start.sh</automated>
  </verify>
  <done>
- `bash -n scripts/start.sh` exits 0 (no syntax errors)
- `grep -c "RedirectStandardOutput" scripts/start.sh` returns >= 1
- `grep -c "RedirectStandardError" scripts/start.sh` returns >= 1
- `grep -c 'WIN_LOG_FILE=$(cygpath' scripts/start.sh` returns exactly 1
- The Unix `nohup "$BINARY" > "$LOG_FILE" 2>&1 &` line is byte-identical to before
- No changes to start.ps1, version strings, or any other file
  </done>
</task>

</tasks>

<verification>
Post-task manual verification (owner: user, after the fix ships in a real SessionStart):
1. Close all Claude Code sessions, confirm dsrcode daemon is stopped (`tasklist | grep dsrcode` empty).
2. Delete or rotate `~/.claude/dsrcode.log` so we can see fresh output.
3. Start a new Claude Code session on Windows.
4. `tail ~/.claude/dsrcode.log` — MUST show slog lines from the newly launched daemon (server start, port bind, health endpoint registration, etc.).
5. If the daemon crashes, `~/.claude/dsrcode.log.err` will contain the stderr trace instead of vanishing.
6. `curl http://127.0.0.1:19460/health` returns 200.
</verification>

<success_criteria>
- scripts/start.sh Windows branch redirects stdout to dsrcode.log and stderr to dsrcode.log.err
- `bash -n scripts/start.sh` passes
- Diff is limited to the Windows launch block (one new line + one modified powershell.exe command) — roughly 4-6 lines changed total
- Unix launch path untouched
- start.ps1 untouched
</success_criteria>

<output>
After completion, commit with message:

```
fix(start.sh): redirect Windows daemon stdout/stderr to dsrcode.log

Windows Start-Process launch had no -RedirectStandardOutput or
-RedirectStandardError, so dsrcode's slog output went to the void and
crashes were silently invisible. Add WIN_LOG_FILE via cygpath -w and
pass both redirect flags to Start-Process. stderr routes to
dsrcode.log.err because PowerShell rejects same-path for both streams
(MutuallyExclusiveArguments).

Unix nohup path is unchanged. start.ps1 has a latent same-path bug on
line 335 — flagged separately, not fixed here.
```

No SUMMARY.md required for this quick task (per quick-mode convention). STATE.md update optional — user will handle.
</output>

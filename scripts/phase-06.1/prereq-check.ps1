#Requires -Version 5.1
<#
.SYNOPSIS
    Phase 6.1 prerequisite checker - verifies no process blocks the directory rename.

.DESCRIPTION
    Validates that no process holds a handle on C:\Users\ktown\Projects\cc-discord-presence
    or runs from within the project directory. Enforces the Windows constraint documented
    in Stack Overflow #41365318: Windows does not permit renaming a directory that is the
    current working directory of any running process.

    Performs 7 checks in order:
      1. dsrcode/cc-discord-presence daemon (PID 55564 - hard-coded current known PID)
      2. Daemon by process name (in case PID rotated)
      3. Claude Code processes (claude.exe, Claude Code.exe) - WARN only, current session expected
      4. Processes with project path in CommandLine/ExecutablePath (via CIM Win32_Process)
      5. Sysinternals Handle.exe file handles (optional dependency, skipped if not installed)
      6. Go build/test/dlv/gopls processes
      7. Claude Code memory dir sanity check (informational)

    The script is idempotent and read-only. It never kills processes, deletes files, or
    modifies state. Safe to run multiple times.

.PARAMETER DryRun
    Run all checks but exit 0 regardless of results. Use for test runs during development
    or when you want to see the full output without gating subsequent operations.

.EXAMPLE
    .\prereq-check.ps1
    Normal invocation. Exit code indicates status (0=ok, 1=warn, 2=block).

.EXAMPLE
    .\prereq-check.ps1 -DryRun
    Report-only mode. Exits 0 regardless of findings.

.EXAMPLE
    .\prereq-check.ps1 -Verbose
    Enable detailed output for each check step.

.NOTES
    Exit codes:
      0 = OK        (all checks clean, safe to run handoff.ps1)
      1 = WARNING   (non-blocking issues, user should review before proceeding)
      2 = BLOCKER   (rename WILL fail, resolve before running handoff.ps1)

    Plan 03 (handoff.ps1) invokes this script via:
      & ./prereq-check.ps1
      if ($LASTEXITCODE -eq 2) { exit 1 }

    Best results when run from an elevated (Run as Administrator) PowerShell session -
    non-elevated sessions cannot read CommandLine for processes owned by other users,
    which creates a blind spot in check #4.

    Phase 6.1 references: D-06 (hybrid prereq check), D-07 (v3.1.10 manual daemon kill).
#>

[CmdletBinding()]
param(
    [switch]$DryRun
)

$ErrorActionPreference = 'Continue'
$ProjectPath = 'C:\Users\ktown\Projects\cc-discord-presence'
$ProjectPattern = '*cc-discord-presence*'
$DaemonPid = 55564

$blockers = New-Object 'System.Collections.Generic.List[string]'
$warnings = New-Object 'System.Collections.Generic.List[string]'

function Write-Check {
    param(
        [Parameter(Mandatory=$true)][string]$Name,
        [Parameter(Mandatory=$true)][ValidateSet('OK','WARN','BLOCK')][string]$Status,
        [string]$Detail
    )
    $color = switch ($Status) {
        'OK'    { 'Green' }
        'WARN'  { 'Yellow' }
        'BLOCK' { 'Red' }
    }
    Write-Host ("[{0,-5}] {1}" -f $Status, $Name) -ForegroundColor $color
    if ($Detail) {
        Write-Host "        $Detail" -ForegroundColor Gray
    }
}

Write-Host ""
Write-Host "Phase 6.1 Prerequisite Check" -ForegroundColor Cyan
Write-Host "Target: $ProjectPath" -ForegroundColor Cyan
Write-Host ("=" * 60) -ForegroundColor Cyan
Write-Host ""

# ---------------------------------------------------------------------------
# Check 1: Daemon PID 55564 (hard-coded current known daemon)
# ---------------------------------------------------------------------------
$daemon = Get-Process -Id $DaemonPid -ErrorAction SilentlyContinue
if ($daemon) {
    $detail = "{0} (PID {1})" -f $daemon.ProcessName, $daemon.Id
    if ($daemon.Path) { $detail += " at $($daemon.Path)" }
    $blockers.Add("Daemon PID ${DaemonPid}: $detail")
    Write-Check -Name "Daemon PID $DaemonPid" -Status 'BLOCK' -Detail $detail
} else {
    Write-Check -Name "Daemon PID $DaemonPid" -Status 'OK' -Detail 'Not running'
}

# ---------------------------------------------------------------------------
# Check 2: Daemon by process name (catches re-spawned daemon with new PID)
# ---------------------------------------------------------------------------
$daemonNames = @(
    'dsrcode',
    'cc-discord-presence',
    'cc-discord-presence-windows-amd64'
)
$daemonByName = Get-Process -Name $daemonNames -ErrorAction SilentlyContinue
if ($daemonByName) {
    $pidList = ($daemonByName | ForEach-Object { "PID $($_.Id) ($($_.ProcessName))" }) -join ', '
    foreach ($p in $daemonByName) {
        $blockers.Add("Daemon process: $($p.ProcessName) PID $($p.Id)")
    }
    Write-Check -Name "Daemon by name" -Status 'BLOCK' -Detail $pidList
} else {
    Write-Check -Name "Daemon by name" -Status 'OK' -Detail 'No dsrcode / cc-discord-presence processes'
}

# ---------------------------------------------------------------------------
# Check 3: Claude Code processes (WARN only - current session is expected)
# ---------------------------------------------------------------------------
# Note: 'Claude Code' contains a space; Get-Process handles this correctly.
# We deliberately exclude 'node' to avoid false positives from other dev tools.
$claudeNames = @('claude', 'Claude Code')
$claudeProcs = Get-Process -Name $claudeNames -ErrorAction SilentlyContinue
if ($claudeProcs) {
    $count = @($claudeProcs).Count
    foreach ($p in $claudeProcs) {
        $warnings.Add("Claude Code process: $($p.ProcessName) PID $($p.Id) (current session may be one of these)")
    }
    Write-Check -Name "Claude Code processes" -Status 'WARN' -Detail "$count running (current session expected; close other Claude windows before handoff)"
} else {
    Write-Check -Name "Claude Code processes" -Status 'OK' -Detail 'None detected by name'
}

# ---------------------------------------------------------------------------
# Check 4: Processes with project path in CommandLine/ExecutablePath (CIM)
# ---------------------------------------------------------------------------
# CIM is the only reliable way to get CommandLine on PS 5.1. Note: non-elevated
# sessions cannot read CommandLine for processes owned by other users, which
# creates a blind spot. We check both CommandLine AND ExecutablePath to widen
# coverage.
try {
    $allProcs = Get-CimInstance -ClassName Win32_Process -ErrorAction Stop
    $cwdProcs = $allProcs | Where-Object {
        $_.ProcessId -ne $PID -and (
            ($_.CommandLine -and $_.CommandLine -like $ProjectPattern) -or
            ($_.ExecutablePath -and $_.ExecutablePath -like $ProjectPattern)
        )
    }
    if ($cwdProcs) {
        foreach ($p in $cwdProcs) {
            $src = if ($p.CommandLine -and $p.CommandLine -like $ProjectPattern) { 'CommandLine' } else { 'ExecutablePath' }
            $blockers.Add("Process with project path in ${src}: $($p.Name) PID $($p.ProcessId)")
        }
        $count = @($cwdProcs).Count
        Write-Check -Name "Processes with project path (CIM)" -Status 'BLOCK' -Detail "$count processes - run elevated for full visibility"
    } else {
        Write-Check -Name "Processes with project path (CIM)" -Status 'OK' -Detail 'None found (Note: non-elevated sessions may miss cross-user processes)'
    }
} catch {
    $errMsg = $_.Exception.Message
    $warnings.Add("CIM query failed: $errMsg")
    Write-Check -Name "Processes with project path (CIM)" -Status 'WARN' -Detail "CIM query failed: $errMsg"
}

# ---------------------------------------------------------------------------
# Check 5: Sysinternals Handle.exe (optional)
# ---------------------------------------------------------------------------
$handleCmd = Get-Command -Name 'handle.exe' -ErrorAction SilentlyContinue
if (-not $handleCmd) {
    $handleCmd = Get-Command -Name 'handle64.exe' -ErrorAction SilentlyContinue
}
if ($handleCmd) {
    try {
        # cmd.exe /c wrapper per The Random Admin (2022) - more reliable output capture
        # than direct & invocation on some Windows versions.
        $handleExe = $handleCmd.Source
        $handleOutput = & cmd.exe /c "`"$handleExe`" -a -u `"$ProjectPath`" -accepteula -nobanner" 2>&1
        $handleLines = @($handleOutput | Where-Object { $_ -and ($_ -like "*$ProjectPath*" -or $_ -like "*pid:*") })
        if ($handleLines.Count -gt 0) {
            $foundPids = New-Object 'System.Collections.Generic.HashSet[string]'
            foreach ($line in $handleLines) {
                if ($line -match 'pid:\s*(\d+)') {
                    [void]$foundPids.Add($matches[1])
                }
            }
            if ($foundPids.Count -gt 0) {
                $pidStr = ($foundPids -join ', ')
                $blockers.Add("Handle.exe detected file locks from PIDs: $pidStr")
                Write-Check -Name "Handle.exe file locks" -Status 'BLOCK' -Detail "Blocking PIDs: $pidStr"
            } else {
                Write-Check -Name "Handle.exe file locks" -Status 'OK' -Detail 'Output matched path but no PIDs parsed'
            }
        } else {
            Write-Check -Name "Handle.exe file locks" -Status 'OK' -Detail 'No locks detected on project path'
        }
    } catch {
        $errMsg = $_.Exception.Message
        $warnings.Add("Handle.exe invocation failed: $errMsg")
        Write-Check -Name "Handle.exe file locks" -Status 'WARN' -Detail "Invocation failed: $errMsg"
    }
} else {
    $warnings.Add('Handle.exe not installed (optional - install from Sysinternals for thorough file-lock detection)')
    Write-Check -Name "Handle.exe" -Status 'WARN' -Detail 'Not installed (CIM fallback is primary; install handle.exe for deeper coverage)'
}

# ---------------------------------------------------------------------------
# Check 6: Go build/test/debug processes
# ---------------------------------------------------------------------------
$goNames = @('go', 'gopls', 'dlv')
$goProcs = Get-Process -Name $goNames -ErrorAction SilentlyContinue
if ($goProcs) {
    $pidList = ($goProcs | ForEach-Object { "$($_.ProcessName) PID $($_.Id)" }) -join ', '
    foreach ($p in $goProcs) {
        $warnings.Add("Go toolchain process: $($p.ProcessName) PID $($p.Id)")
    }
    Write-Check -Name "Go processes (build/test/dlv/gopls)" -Status 'WARN' -Detail $pidList
} else {
    Write-Check -Name "Go processes (build/test/dlv/gopls)" -Status 'OK' -Detail 'None running'
}

# ---------------------------------------------------------------------------
# Check 7: Claude Code memory dir (informational sanity check)
# ---------------------------------------------------------------------------
$memDir = Join-Path $env:USERPROFILE '.claude\projects\C--Users-ktown-Projects-cc-discord-presence'
if (Test-Path $memDir) {
    Write-Check -Name "Memory dir (old path)" -Status 'OK' -Detail "Exists - will be migrated by handoff.ps1"
} else {
    $warnings.Add("Memory dir not found at expected path: $memDir (unusual state, investigate)")
    Write-Check -Name "Memory dir (old path)" -Status 'WARN' -Detail "Not found at $memDir (unusual - investigate)"
}

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
Write-Host ""
Write-Host ("=" * 60) -ForegroundColor Cyan
Write-Host "Summary" -ForegroundColor Cyan

$blockerColor = if ($blockers.Count -gt 0) { 'Red' } else { 'Green' }
$warningColor = if ($warnings.Count -gt 0) { 'Yellow' } else { 'Green' }
Write-Host "  Blockers: $($blockers.Count)" -ForegroundColor $blockerColor
Write-Host "  Warnings: $($warnings.Count)" -ForegroundColor $warningColor

if ($blockers.Count -gt 0) {
    Write-Host ""
    Write-Host "BLOCKERS (must resolve before handoff):" -ForegroundColor Red
    foreach ($b in $blockers) {
        Write-Host "  - $b" -ForegroundColor Red
    }
}

if ($warnings.Count -gt 0) {
    Write-Host ""
    Write-Host "WARNINGS (review before proceeding):" -ForegroundColor Yellow
    foreach ($w in $warnings) {
        Write-Host "  - $w" -ForegroundColor Yellow
    }
}

Write-Host ""

if ($DryRun) {
    Write-Host "[DryRun] Exit code forced to 0 regardless of findings." -ForegroundColor Cyan
    exit 0
}

if ($blockers.Count -gt 0) {
    Write-Host "Exit 2: BLOCKERS present. DO NOT run handoff.ps1 yet." -ForegroundColor Red
    exit 2
}

if ($warnings.Count -gt 0) {
    Write-Host "Exit 1: Warnings present. Review above before proceeding." -ForegroundColor Yellow
    exit 1
}

Write-Host "Exit 0: All checks passed. Safe to run handoff.ps1." -ForegroundColor Green
exit 0

#Requires -Version 5.1
<#
.SYNOPSIS
    Phase 6.1 handoff execution — rename project dir + migrate Claude memory + reinstall plugin.

.DESCRIPTION
    Executes all 10 handoff steps in sequence. MUST be run from an external shell
    (NOT from within the Claude Code session that has cwd in the project directory).

.PARAMETER DryRun
    Simulate all steps without making any changes (prints what would happen).

.PARAMETER SkipPrereqCheck
    Skip Step 0 (prereq-check.ps1 invocation). Use only if you've already verified manually.

.EXAMPLE
    .\handoff.ps1
    .\handoff.ps1 -DryRun
    .\handoff.ps1 -SkipPrereqCheck

.NOTES
    Execution order (LOAD-BEARING — do not reorder):
      0. Prereq-check (exit 2 halts execution)
      1. Stop daemon PID 55564 (manual kill since v3.1.10 has no auto-exit)
      2. Delete legacy binary ~/.claude/bin/cc-discord-presence-windows-amd64.exe
      3. Rename project dir: cc-discord-presence → dsrcode
      4. Rename Claude memory dir: C--Users-ktown-Projects-cc-discord-presence → dsrcode variant
      5. Marketplace git pull (workaround for #41885, #37172, #29071)
      6. Remove orphan cache dirs 3.1.9 + 3.1.10 (workaround for #17361, #29074, #19197)
      7. claude plugin uninstall dsrcode@dsrcode --keep-data (D-12 mandatory)
      8. claude plugin install dsrcode@dsrcode (fresh download from updated marketplace)
      9. Instruct user to start NEW Claude Code session in NEW dir
#>

[CmdletBinding()]
param(
    [switch]$DryRun,
    [switch]$SkipPrereqCheck
)

$ErrorActionPreference = 'Stop'
$OldPath    = 'C:\Users\ktown\Projects\cc-discord-presence'
$NewPath    = 'C:\Users\ktown\Projects\dsrcode'
$OldMemDir  = "$env:USERPROFILE\.claude\projects\C--Users-ktown-Projects-cc-discord-presence"
$NewMemDir  = "$env:USERPROFILE\.claude\projects\C--Users-ktown-Projects-dsrcode"
$LegacyBin  = "$env:USERPROFILE\.claude\bin\cc-discord-presence-windows-amd64.exe"
$MktClone   = "$env:USERPROFILE\.claude\plugins\marketplaces\dsrcode"
$CacheBase  = "$env:USERPROFILE\.claude\plugins\cache\dsrcode\dsrcode"

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Action,
        [string]$DryRunDescription
    )
    Write-Host ""
    Write-Host "─── Step: $Name ───" -ForegroundColor Cyan
    if ($DryRun) {
        Write-Host "[DRY] $DryRunDescription" -ForegroundColor Yellow
        return
    }
    try {
        & $Action
        Write-Host "✓ $Name complete" -ForegroundColor Green
    } catch {
        Write-Host "✗ $Name FAILED: $($_.Exception.Message)" -ForegroundColor Red
        Write-Host "  See HANDOFF.md 'Rollback' section for recovery." -ForegroundColor Yellow
        throw
    }
}

Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Phase 6.1 Handoff — $(if ($DryRun) { 'DRY RUN' } else { 'LIVE EXECUTION' })" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

# Step 0: Prereq check
if (-not $SkipPrereqCheck) {
    Invoke-Step -Name "0. Prereq check" -DryRunDescription "Would run prereq-check.ps1" -Action {
        & "$PSScriptRoot\prereq-check.ps1"
        if ($LASTEXITCODE -eq 2) {
            throw "prereq-check.ps1 reported BLOCKERS (exit 2). Resolve before continuing."
        }
        if ($LASTEXITCODE -eq 1) {
            Write-Host "  Warnings present. Press ENTER to continue or Ctrl+C to abort."
            if (-not $DryRun) { Read-Host }
        }
    }
}

# Step 1: Stop daemon PID 55564 (D-07)
Invoke-Step -Name "1. Stop daemon (PID 55564)" -DryRunDescription "Would run: Stop-Process -Id 55564 -Force" -Action {
    $daemon = Get-Process -Id 55564 -ErrorAction SilentlyContinue
    if ($daemon) {
        Stop-Process -Id 55564 -Force
        Start-Sleep -Seconds 2   # Give process time to exit before filesystem operations
        $stillRunning = Get-Process -Id 55564 -ErrorAction SilentlyContinue
        if ($stillRunning) { throw "Daemon still running after Stop-Process -Force" }
    } else {
        Write-Host "  Daemon not running (OK)"
    }
}

# Step 2: Delete legacy binary (D-10)
Invoke-Step -Name "2. Delete legacy binary" -DryRunDescription "Would delete $LegacyBin" -Action {
    if (Test-Path $LegacyBin) {
        Remove-Item -Path $LegacyBin -Force
    } else {
        Write-Host "  Legacy binary not present (already cleaned)"
    }
}

# Step 3: Rename project dir (D-01, D-03, D-04)
Invoke-Step -Name "3. Rename project directory" -DryRunDescription "Would rename $OldPath → $NewPath" -Action {
    if (Test-Path $NewPath) {
        throw "New path $NewPath already exists. Aborting to prevent data loss."
    }
    if (-not (Test-Path $OldPath)) {
        throw "Old path $OldPath does not exist. Already renamed?"
    }
    Rename-Item -Path $OldPath -NewName 'dsrcode' -ErrorAction Stop
}

# Step 4: Rename memory dir (D-02, D-05)
Invoke-Step -Name "4. Rename Claude memory directory" -DryRunDescription "Would rename $OldMemDir → $NewMemDir" -Action {
    if (Test-Path $NewMemDir) {
        throw "New memory dir $NewMemDir already exists. Aborting."
    }
    if (Test-Path $OldMemDir) {
        Rename-Item -Path $OldMemDir -NewName 'C--Users-ktown-Projects-dsrcode' -ErrorAction Stop
    } else {
        Write-Host "  Old memory dir not present (unusual but not fatal)"
    }
}

# Step 5: Marketplace git pull (D-08 step 3 — workaround #41885, #37172, #29071)
Invoke-Step -Name "5. Marketplace git pull" -DryRunDescription "Would run git pull in $MktClone" -Action {
    if (-not (Test-Path $MktClone)) {
        throw "Marketplace clone not found at $MktClone"
    }
    Push-Location $MktClone
    try {
        & git pull origin main
        if ($LASTEXITCODE -ne 0) { throw "git pull failed (exit $LASTEXITCODE)" }
    } finally {
        Pop-Location
    }
}

# Step 6: Remove orphan cache dirs (D-13 — workaround #17361, #29074, #19197)
Invoke-Step -Name "6. Remove orphan plugin cache dirs" -DryRunDescription "Would rm -rf $CacheBase\3.1.9 and 3.1.10" -Action {
    foreach ($ver in @('3.1.9', '3.1.10')) {
        $dir = Join-Path $CacheBase $ver
        if (Test-Path $dir) {
            Remove-Item -Recurse -Force $dir
            Write-Host "  Removed $dir"
        } else {
            Write-Host "  $dir not present (already cleaned)"
        }
    }
}

# Step 7: Plugin uninstall --keep-data (D-12 mandatory)
Invoke-Step -Name "7. Uninstall plugin (keep-data)" -DryRunDescription "Would run: claude plugin uninstall dsrcode@dsrcode --keep-data" -Action {
    & claude plugin uninstall dsrcode@dsrcode --keep-data
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  Warning: uninstall exit code $LASTEXITCODE — plugin may not have been installed, continuing" -ForegroundColor Yellow
    }
}

# Step 8: Plugin install (D-08 step 6 — fresh download)
Invoke-Step -Name "8. Install plugin (fresh download)" -DryRunDescription "Would run: claude plugin install dsrcode@dsrcode" -Action {
    & claude plugin install dsrcode@dsrcode
    if ($LASTEXITCODE -ne 0) {
        throw "Plugin install failed (exit $LASTEXITCODE). See HANDOFF.md troubleshooting."
    }
}

# Step 9: Final instruction
Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host " Handoff complete!" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  1. Close THIS shell window."
Write-Host "  2. Open a NEW Claude Code session in:"
Write-Host "     cd $NewPath && claude"
Write-Host "  3. In the new session, run Phase 6.1 Plan 05 OR verify.ps1 directly:"
Write-Host "     .\scripts\phase-06.1\verify.ps1"
Write-Host ""
Write-Host "  DO NOT use the old directory path anymore." -ForegroundColor Yellow
Write-Host "  Legacy binary, cache orphans, and memory dir have been migrated." -ForegroundColor Yellow

if ($DryRun) {
    Write-Host ""
    Write-Host "[DRY RUN COMPLETE] No changes were made." -ForegroundColor Cyan
}

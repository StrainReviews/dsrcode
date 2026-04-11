#Requires -Version 5.1
<#
.SYNOPSIS
    Phase 6.1 post-rename verification — runs T1-T8 smoke tests.

.DESCRIPTION
    Verifies the dsrcode migration succeeded end-to-end. Runs in the NEW Claude Code session
    from the renamed directory. Tests the binary version, daemon health, hooks, auto-exit,
    and plugin cache state. Includes manual prompts for Discord/tool-use/memory checks.

.PARAMETER SkipOptional
    Skip optional tests T9 (error icon) and T10 (subagent spawn).

.PARAMETER SkipManual
    Skip manual prompt tests (T3, T5, T6) — useful for CI-like unattended runs.

.EXAMPLE
    .\verify.ps1
    .\verify.ps1 -SkipOptional
    .\verify.ps1 -SkipManual -SkipOptional

.NOTES
    Exit codes:
      0 = All attempted tests passed
      1 = One or more tests failed
      2 = Script error (not test failure)
#>

[CmdletBinding()]
param(
    [switch]$SkipOptional,
    [switch]$SkipManual
)

$ErrorActionPreference = 'Continue'

# Test result tracking
$script:results = [ordered]@{}
$script:passCount = 0
$script:failCount = 0
$script:skipCount = 0

function Invoke-Test {
    param(
        [string]$Id,
        [string]$Description,
        [scriptblock]$Test
    )
    Write-Host -NoNewline "[$Id] $Description ... "
    try {
        $result = & $Test
        if ($result -is [bool] -and $result) {
            Write-Host "PASS" -ForegroundColor Green
            $script:passCount++
            $script:results[$Id] = 'PASS'
            return $true
        } else {
            Write-Host "FAIL" -ForegroundColor Red
            $script:failCount++
            $script:results[$Id] = 'FAIL'
            return $false
        }
    } catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
        $script:failCount++
        $script:results[$Id] = "ERROR: $($_.Exception.Message)"
        return $false
    }
}

function Read-ManualAssertion {
    param([string]$Id, [string]$Question)
    if ($SkipManual) {
        Write-Host "[$Id] SKIP (manual test, -SkipManual set)" -ForegroundColor DarkGray
        $script:skipCount++
        $script:results[$Id] = 'SKIP'
        return
    }
    Write-Host ""
    Write-Host "[$Id] MANUAL CHECK:" -ForegroundColor Cyan
    Write-Host "     $Question" -ForegroundColor Cyan
    $answer = Read-Host "     (y/n)"
    if ($answer -match '^[yY]') {
        Write-Host "[$Id] PASS" -ForegroundColor Green
        $script:passCount++
        $script:results[$Id] = 'PASS'
    } else {
        Write-Host "[$Id] FAIL" -ForegroundColor Red
        $script:failCount++
        $script:results[$Id] = 'FAIL'
    }
}

Write-Host "======================================================"
Write-Host " Phase 6.1 Verification — T1-T8 Smoke Tests"
Write-Host "======================================================"
Write-Host ""

# T1: Binary version
Invoke-Test -Id 'T1' -Description 'dsrcode --version returns 4.1.0' -Test {
    $dsrcodeBin = Get-Command -Name 'dsrcode' -ErrorAction SilentlyContinue
    if (-not $dsrcodeBin) { return $false }
    $output = & dsrcode --version 2>&1 | Out-String
    return ($output -match 'dsrcode\s+4\.1\.0')
}

# T2: Health endpoint
Invoke-Test -Id 'T2' -Description 'HTTP 200 from http://127.0.0.1:19460/health' -Test {
    try {
        $resp = Invoke-WebRequest -Uri 'http://127.0.0.1:19460/health' -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
        return ($resp.StatusCode -eq 200)
    } catch {
        return $false
    }
}

# T3: Discord presence (manual)
Read-ManualAssertion -Id 'T3' -Question "Open Discord. Do you see Rich Presence for 'dsrcode' with git branch and activity icon?"

# T4: 13 HTTP hooks in settings.local.json
Invoke-Test -Id 'T4' -Description 'settings.local.json has 13 http hooks at 127.0.0.1:19460' -Test {
    $settingsPath = "$env:USERPROFILE\.claude\settings.local.json"
    if (-not (Test-Path $settingsPath)) { return $false }
    $settings = Get-Content $settingsPath -Raw | ConvertFrom-Json
    if (-not $settings.hooks) { return $false }
    $hookCount = 0
    foreach ($eventName in $settings.hooks.PSObject.Properties.Name) {
        $matchers = $settings.hooks.$eventName
        foreach ($matcher in $matchers) {
            if ($matcher.hooks) {
                foreach ($hook in $matcher.hooks) {
                    if ($hook.type -eq 'http' -and $hook.url -like '*127.0.0.1:19460*') {
                        $hookCount++
                    }
                }
            }
        }
    }
    Write-Host -NoNewline "(found $hookCount/13) "
    return ($hookCount -eq 13)
}

# T5: Tool-use triggers presence update (manual)
Read-ManualAssertion -Id 'T5' -Question "In Claude Code, run a Read tool on any file. Does Discord presence update within 15 seconds?"

# T6: Auto-memory loaded (manual)
Read-ManualAssertion -Id 'T6' -Question "In Claude Code, ask 'what do you remember about this project'. Does it cite MEMORY.md entries?"

# T7: Auto-exit grace period
Write-Host ""
Write-Host "[T7] Close all Claude Code sessions EXCEPT this one, then press ENTER." -ForegroundColor Cyan
Write-Host "     (If this script is running outside Claude Code, close all Claude Code sessions and press ENTER.)" -ForegroundColor Cyan
if (-not $SkipManual) { Read-Host "     Continue" | Out-Null }
Write-Host "     Waiting 35 seconds for auto-exit grace period..." -ForegroundColor Cyan
Start-Sleep -Seconds 35
Invoke-Test -Id 'T7' -Description 'Daemon auto-exited (dsrcode.pid absent after 35s)' -Test {
    $pidFile = "$env:USERPROFILE\.claude\dsrcode.pid"
    return (-not (Test-Path $pidFile))
}

# T8: Cache has only 4.1.0/
Invoke-Test -Id 'T8' -Description 'Plugin cache has only 4.1.0/ subdirectory' -Test {
    $cacheDir = "$env:USERPROFILE\.claude\plugins\cache\dsrcode\dsrcode"
    if (-not (Test-Path $cacheDir)) {
        Write-Host -NoNewline "(cache dir not found) "
        return $false
    }
    $versions = Get-ChildItem -Path $cacheDir -Directory | Select-Object -ExpandProperty Name
    Write-Host -NoNewline "(found: $($versions -join ', ')) "
    return ($versions.Count -eq 1 -and $versions[0] -eq '4.1.0')
}

# T9 (optional): Error icon
if (-not $SkipOptional) {
    Write-Host ""
    Write-Host "[T9] OPTIONAL: Error icon test — force a Claude API error (e.g., invalid model)." -ForegroundColor Cyan
    Read-ManualAssertion -Id 'T9' -Question "Does Discord SmallImage show the 'error' icon after the API error?"
}

# T10 (optional): Subagent spawn
if (-not $SkipOptional) {
    Write-Host ""
    Write-Host "[T10] OPTIONAL: Spawn any subagent via Task tool." -ForegroundColor Cyan
    Read-ManualAssertion -Id 'T10' -Question "Does Discord presence show 'thinking' icon with subagent info?"
}

# Summary
Write-Host ""
Write-Host "======================================================"
Write-Host " Results"
Write-Host "======================================================"
Write-Host "  PASS: $passCount" -ForegroundColor Green
Write-Host "  FAIL: $failCount" -ForegroundColor ($(if ($failCount -gt 0) { 'Red' } else { 'Green' }))
Write-Host "  SKIP: $skipCount" -ForegroundColor DarkGray
Write-Host ""

# Detailed table
$results.GetEnumerator() | ForEach-Object {
    $color = switch -Wildcard ($_.Value) {
        'PASS'  { 'Green' }
        'FAIL'  { 'Red' }
        'SKIP'  { 'DarkGray' }
        'ERROR*' { 'Red' }
        default { 'White' }
    }
    Write-Host ("  {0,-4} {1}" -f $_.Key, $_.Value) -ForegroundColor $color
}

Write-Host ""

# Export results to JSON for Plan 05 SUMMARY consumption
$summaryPath = Join-Path $PSScriptRoot 'verify-results.json'
$summary = @{
    phase       = '6.1'
    timestamp   = (Get-Date).ToString('o')
    passCount   = $passCount
    failCount   = $failCount
    skipCount   = $skipCount
    results     = $results
} | ConvertTo-Json -Depth 4
Set-Content -Path $summaryPath -Value $summary -Encoding UTF8
Write-Host "Results saved to: $summaryPath" -ForegroundColor Gray

if ($failCount -gt 0) {
    Write-Host ""
    Write-Host "Exit 1: $failCount test(s) failed. Review results above." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Exit 0: All $passCount attempted tests passed." -ForegroundColor Green
exit 0

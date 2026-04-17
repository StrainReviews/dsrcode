#Requires -Version 5.1
<#
.SYNOPSIS
    Phase 8 live-daemon verification harness — runs T1-T6 cross-platform smoke tests.

.DESCRIPTION
    Validates:
      T1 dsrcode binary reports version 4.2.0
      T2 /health returns HTTP 200
      T3 Burst 10 distinct signals -> <=3 coalesced SetActivity calls observed
      T4 Burst 10 identical signals -> >=9 content_hash skip events observed
      T5 Duplicate POST /hooks/pre-tool-use within 500ms -> exactly 1 hook deduped log
      T6 60s summary log appears after activity

    Preconditions: Daemon must already be running (start.ps1 launches it).

.EXAMPLE
    .\scripts\phase-08\verify.ps1
#>

[CmdletBinding()]
param()

$ErrorActionPreference = 'Continue'

# Test result tracking
$script:results  = [ordered]@{}
$script:passCount = 0
$script:failCount = 0

$DaemonUrl = 'http://127.0.0.1:19460'
$LogFile   = Join-Path $env:USERPROFILE '.claude\dsrcode.log'

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

# Record log offset so T3-T6 only scan lines written during this run.
if (-not (Test-Path -Path $LogFile)) {
    Write-Host "FAIL: log file $LogFile not found -- daemon not running?" -ForegroundColor Red
    exit 1
}
$script:logOffset = (Get-Item $LogFile).Length

function Get-NewLogBytes {
    $fs = [System.IO.File]::Open($LogFile, 'Open', 'Read', 'ReadWrite')
    try {
        $null = $fs.Seek($script:logOffset, 'Begin')
        $sr = New-Object System.IO.StreamReader($fs)
        return $sr.ReadToEnd()
    } finally {
        $fs.Close()
    }
}

function Count-LogMatches {
    param([string]$Pattern)
    $text = Get-NewLogBytes
    if ([string]::IsNullOrEmpty($text)) { return 0 }
    return ([regex]::Matches($text, $Pattern)).Count
}

function Invoke-HookPost {
    param(
        [string]$Path,
        [string]$Body
    )
    Invoke-WebRequest -Method Post -Uri ($DaemonUrl + $Path) `
        -ContentType 'application/json' -Body $Body `
        -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop | Out-Null
}

# Registered session_ids for end-of-run cleanup. Populated from inside each
# test's scriptblock BEFORE the pre-tool-use POST, so a mid-script interrupt
# (Ctrl+C, thrown exception, exit) still fires session-end for every fake
# session the daemon saw. The finally block below walks this list and posts
# /hooks/session-end for each ID — per-entry try/catch so one failing POST
# cannot skip the rest (Microsoft Learn about_Try_Catch_Finally).
$script:CleanupIds = New-Object 'System.Collections.Generic.List[string]'

try {

# --- T1: Binary version ---
Invoke-Test -Id 'T1' -Description 'dsrcode --version reports 4.2.0' -Test {
    $cmd = Get-Command -Name 'dsrcode' -ErrorAction SilentlyContinue
    if (-not $cmd) { return $false }
    $output = & dsrcode --version 2>&1 | Out-String
    return ($output -match '4\.2\.0')
}

# --- T2: /health ---
Invoke-Test -Id 'T2' -Description 'HTTP 200 from /health' -Test {
    try {
        $resp = Invoke-WebRequest -Uri ($DaemonUrl + '/health') -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop
        return ($resp.StatusCode -eq 200)
    } catch {
        return $false
    }
}

# --- T3: Burst coalescing ---
Invoke-Test -Id 'T3' -Description 'Burst 10 distinct signals produce <=3 coalesced flushes' -Test {
    for ($i = 1; $i -le 10; $i++) {
        $script:CleanupIds.Add("t3-s$i")
        Invoke-HookPost -Path '/hooks/pre-tool-use' `
            -Body "{`"session_id`":`"t3-s$i`",`"tool_name`":`"Edit`",`"cwd`":`"/tmp/t3`"}"
    }
    Start-Sleep -Seconds 10
    $count = Count-LogMatches '"presence updated"'
    Write-Verbose "T3 observed $count SetActivity calls"
    return ($count -le 3)
}

# --- T4: Content-hash skip ---
Invoke-Test -Id 'T4' -Description 'Burst 10 identical signals produce >=9 content_hash skips' -Test {
    $script:CleanupIds.Add("t4-s1")
    $body = '{"session_id":"t4-s1","tool_name":"Edit","cwd":"/tmp/t4"}'
    for ($i = 1; $i -le 10; $i++) {
        try { Invoke-HookPost -Path '/hooks/pre-tool-use' -Body $body } catch {}
        Start-Sleep -Milliseconds 600
    }
    Start-Sleep -Seconds 2
    $count = Count-LogMatches '"presence update skipped".*"reason":"content_hash"'
    Write-Verbose "T4 observed $count content_hash skips"
    return ($count -ge 9)
}

# --- T5: Hook dedup ---
Invoke-Test -Id 'T5' -Description '2 identical POSTs within 500ms produce exactly 1 hook dedup log' -Test {
    $script:CleanupIds.Add("t5-s1")
    $body = '{"session_id":"t5-s1","tool_name":"Grep","cwd":"/tmp/t5"}'
    Invoke-HookPost -Path '/hooks/pre-tool-use' -Body $body
    Invoke-HookPost -Path '/hooks/pre-tool-use' -Body $body
    Start-Sleep -Seconds 1
    $count = Count-LogMatches '"hook deduped"'
    Write-Verbose "T5 observed $count hook dedups"
    return ($count -eq 1)
}

# --- T6: 60s summary log ---
Invoke-Test -Id 'T6' -Description '60s coalescer-status summary appears' -Test {
    Write-Verbose 'T6 waiting 65s for summary...'
    Start-Sleep -Seconds 65
    $count = Count-LogMatches '"coalescer status"'
    Write-Verbose "T6 observed $count summary logs"
    return ($count -ge 1)
}

# --- Summary ---
Write-Host ''
Write-Host "Phase 8 verify: $script:passCount passed, $script:failCount failed"

$summaryPath = Join-Path $PSScriptRoot 'verify-results.json'
$summary = @{
    phase     = '8'
    timestamp = (Get-Date).ToString('o')
    passCount = $script:passCount
    failCount = $script:failCount
    results   = $script:results
} | ConvertTo-Json -Depth 4
Set-Content -Path $summaryPath -Value $summary -Encoding UTF8

if ($script:failCount -gt 0) { exit 1 }
exit 0

}
finally {
    # Cleanup: fire /hooks/session-end for every fake session_id injected by
    # T3..T5 so the daemon registry does not carry phantom entries past this
    # script's lifetime. Runs on normal exit, uncaught exception, `exit`,
    # and Ctrl+C (per about_Try_Catch_Finally PS 5.1). Per-entry try/catch
    # so a failing POST cannot abort the cleanup loop.
    foreach ($id in $script:CleanupIds) {
        try {
            $body = "{`"session_id`":`"$id`",`"reason`":`"verify-ps1-cleanup`"}"
            Invoke-WebRequest -Method Post -Uri ($DaemonUrl + '/hooks/session-end') `
                -ContentType 'application/json' -Body $body `
                -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop | Out-Null
        } catch {
            # Swallow — cleanup must never abort. Phantom session for this ID
            # will time out via Phase 3 stale-session cleanup eventually.
        }
    }
}

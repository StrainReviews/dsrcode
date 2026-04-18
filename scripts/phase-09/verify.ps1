#Requires -Version 5.1
<#
.SYNOPSIS
    Phase 9 live-daemon verification harness (v4.2.1 hotfix).

.DESCRIPTION
    Validates:
      T1 dsrcode binary reports version 4.2.1
      T2 /health returns HTTP 200
      T3 SourceClaude UUID session survives >=150s stale-check sweep
         (0 "removing stale session" lines emitted for the injected UUID)

    Preconditions:
      - Daemon must already be running (start.ps1 launches it).
      - dsrcode.exe must be on PATH.
      - $env:USERPROFILE\.claude\dsrcode.log must exist.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\phase-09\verify.ps1
#>

[CmdletBinding()]
param()

$ErrorActionPreference = 'Continue'

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

$script:CleanupIds = New-Object 'System.Collections.Generic.List[string]'

try {

# --- T1: Binary version ---
Invoke-Test -Id 'T1' -Description 'dsrcode --version reports 4.2.1' -Test {
    $cmd = Get-Command -Name 'dsrcode' -ErrorAction SilentlyContinue
    if (-not $cmd) { return $false }
    $output = & dsrcode --version 2>&1 | Out-String
    return ($output -match '4\.2\.1')
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

# --- T3: SourceClaude UUID session survives >=150s stale-check sweep ---
Invoke-Test -Id 'T3' -Description 'SourceClaude UUID session survives PID-liveness sweep' -Test {
    $uuid = "verify09-$(Get-Date -Format 'yyyyMMddHHmmss')-abc-def-1234-5678-90ab-cdef"
    $script:CleanupIds.Add($uuid)
    Invoke-HookPost -Path '/hooks/pre-tool-use' `
        -Body "{`"session_id`":`"$uuid`",`"tool_name`":`"Edit`",`"cwd`":`"/tmp/verify09`"}"

    Write-Host 'T3 waiting 150s for stale-check ticker to sweep (mirrors incident elapsed)...'
    Start-Sleep -Seconds 150

    $pattern = '"removing stale session".*"' + [regex]::Escape($uuid) + '"'
    $removes = Count-LogMatches $pattern
    Write-Verbose "T3 observed $removes removals for $uuid (expect 0)"
    return ($removes -eq 0)
}

# --- Summary ---
Write-Host ''
Write-Host "Phase 9 verify: $script:passCount passed, $script:failCount failed"

$summaryPath = Join-Path $PSScriptRoot 'verify-results.json'
$summary = @{
    phase     = '9'
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
    foreach ($id in $script:CleanupIds) {
        try {
            $body = "{`"session_id`":`"$id`",`"reason`":`"verify-09-ps1-cleanup`"}"
            Invoke-WebRequest -Method Post -Uri ($DaemonUrl + '/hooks/session-end') `
                -ContentType 'application/json' -Body $body `
                -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop | Out-Null
        } catch {
            # Swallow — cleanup must never abort.
        }
    }
}

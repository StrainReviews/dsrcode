#!/usr/bin/env pwsh
# Test harness for Phase 7 Bug #4 Rotate-Log function.
# Validates: 11MB file gets renamed to .log.1; second 11MB run overwrites .log.1.
# Usage: pwsh scripts/test-rotate-log.ps1
# Exits 0 on success, non-zero on any assertion failure.

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$StartPs1 = Join-Path $ScriptDir "start.ps1"

if (-not (Test-Path $StartPs1)) {
    Write-Error "FAIL: $StartPs1 not found"
    exit 1
}

# Extract the Rotate-Log function definition from start.ps1 and dot-source it
# into the current scope. Use a regex to find the function block.
$content = Get-Content -Raw -Path $StartPs1
$pattern = '(?ms)^function Rotate-Log\s*\{.*?^\}'
$regexMatch = [regex]::Match($content, $pattern)
if (-not $regexMatch.Success) {
    Write-Error "FAIL: Rotate-Log function not found in $StartPs1"
    exit 1
}
$rotateLogCode = $regexMatch.Value

# Define the function in current scope by invoking the extracted code.
Invoke-Expression $rotateLogCode

# Set up tmp test directory.
$TmpDir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "dsrcode-rotate-test-$PID")
try {
    $TestLog = Join-Path $TmpDir "test.log"

    # Test 1: 5MB file — should NOT rotate.
    $bytes = New-Object byte[] (5 * 1024 * 1024)
    [System.IO.File]::WriteAllBytes($TestLog, $bytes)
    Rotate-Log $TestLog
    if (Test-Path "$TestLog.1") {
        Write-Error "FAIL: 5MB file should NOT have been rotated"
        exit 1
    }
    if (-not (Test-Path $TestLog)) {
        Write-Error "FAIL: 5MB file should still exist"
        exit 1
    }
    Write-Host "PASS: 5MB file not rotated"

    # Test 2: 11MB file — should rotate to .log.1.
    $bytes = New-Object byte[] (11 * 1024 * 1024)
    [System.IO.File]::WriteAllBytes($TestLog, $bytes)
    Rotate-Log $TestLog
    if (-not (Test-Path "$TestLog.1")) {
        Write-Error "FAIL: 11MB file should have been rotated to .log.1"
        exit 1
    }
    if (Test-Path $TestLog) {
        Write-Error "FAIL: original 11MB .log file should be gone after rotation"
        exit 1
    }
    $size1 = (Get-Item "$TestLog.1").Length
    if ($size1 -lt 11000000) {
        Write-Error "FAIL: .log.1 should hold ~11MB; got $size1 bytes"
        exit 1
    }
    Write-Host "PASS: 11MB file rotated to .log.1"

    # Test 3: second rotation — .log.1 overwritten, no .log.2.
    [System.IO.File]::WriteAllBytes($TestLog, $bytes)
    Rotate-Log $TestLog
    if (Test-Path "$TestLog.2") {
        Write-Error "FAIL: .log.2 should NOT exist (single-backup retention)"
        exit 1
    }
    if (-not (Test-Path "$TestLog.1")) {
        Write-Error "FAIL: .log.1 should still exist after second rotation"
        exit 1
    }
    Write-Host "PASS: second rotation overwrote .log.1 (no .log.2)"

    # Test 4: non-existent file — should be a no-op.
    Remove-Item -Path $TestLog,"$TestLog.1" -Force -ErrorAction SilentlyContinue
    try {
        Rotate-Log $TestLog
    } catch {
        Write-Error "FAIL: Rotate-Log should not throw for missing file: $_"
        exit 1
    }
    Write-Host "PASS: Rotate-Log no-op for missing file"

    # Test 5: empty file — should NOT rotate.
    New-Item -ItemType File -Path $TestLog -Force | Out-Null
    Rotate-Log $TestLog
    if (Test-Path "$TestLog.1") {
        Write-Error "FAIL: empty file should NOT be rotated"
        exit 1
    }
    Write-Host "PASS: empty file not rotated"

    Write-Host ""
    Write-Host "All Rotate-Log tests passed."
} finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}

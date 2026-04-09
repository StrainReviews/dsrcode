# Stop dsrcode daemon (Discord Rich Presence for Claude Code)
# WARNING: Windows support is untested. Please report issues on GitHub.

# Configuration -- new paths with fallback to old names during migration
$ClaudeDir = Join-Path $env:USERPROFILE ".claude"
$PidFile = Join-Path $ClaudeDir "dsrcode.pid"
$OldPidFile = Join-Path $ClaudeDir "discord-presence.pid"
$RefcountFile = Join-Path $ClaudeDir "dsrcode.refcount"
$OldRefcountFile = Join-Path $ClaudeDir "discord-presence.refcount"

# Session tracking: Use refcount (PID-based tracking is unreliable on Windows)
# Try new refcount file first, fall back to old
$CurrentCount = 1
if (Test-Path $RefcountFile) {
    $CurrentCount = [int](Get-Content $RefcountFile -ErrorAction SilentlyContinue)
} elseif (Test-Path $OldRefcountFile) {
    $CurrentCount = [int](Get-Content $OldRefcountFile -ErrorAction SilentlyContinue)
}
$ActiveSessions = [Math]::Max(0, $CurrentCount - 1)

if ($ActiveSessions -gt 0) {
    # Write decremented count to new path (migrate forward)
    $ActiveSessions | Out-File -FilePath $RefcountFile -Encoding ASCII -NoNewline
    # Clean up old file if it exists
    Remove-Item $OldRefcountFile -Force -ErrorAction SilentlyContinue
    Write-Host "dsrcode still in use by $ActiveSessions session(s)"
    exit 0
}

# No more sessions, clean up both refcount files
Remove-Item $RefcountFile -Force -ErrorAction SilentlyContinue
Remove-Item $OldRefcountFile -Force -ErrorAction SilentlyContinue

# Stop the daemon
# Try new PID file first, then old
$ActualPidFile = $null
if (Test-Path $PidFile) { $ActualPidFile = $PidFile }
elseif (Test-Path $OldPidFile) { $ActualPidFile = $OldPidFile }

if ($ActualPidFile) {
    $ProcessId = Get-Content $ActualPidFile -ErrorAction SilentlyContinue
    if ($ProcessId) {
        $Process = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
        if ($Process) {
            Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
            Write-Host "dsrcode stopped (PID: $ProcessId)"
        }
    }
    # Clean up both PID files
    Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
    Remove-Item $OldPidFile -Force -ErrorAction SilentlyContinue
} else {
    # Try to find and kill by process name
    $Processes = Get-Process -Name "dsrcode*" -ErrorAction SilentlyContinue
    if (-not $Processes) {
        # Also try old name during migration period
        $Processes = Get-Process -Name "cc-discord-presence*" -ErrorAction SilentlyContinue
    }
    if ($Processes) {
        $Processes | Stop-Process -Force
        Write-Host "dsrcode stopped"
    }
}

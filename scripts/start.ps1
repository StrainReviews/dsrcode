# Start dsrcode daemon (Discord Rich Presence for Claude Code) - Windows
# Downloads pre-built binary from GitHub Releases, falls back to go build.
# WARNING: Windows support is untested. Please report issues on GitHub.

$ErrorActionPreference = "Continue"  # Not "Stop" -- download failures are expected flow control

# ---- Configuration ----
$ClaudeDir = Join-Path $env:USERPROFILE ".claude"
$Repo = "StrainReviews/dsrcode"
$Version = "v4.2.3"

# Binary storage: CLAUDE_PLUGIN_DATA (official, persistent) with fallback per DIST-19
$PluginData = if ($env:CLAUDE_PLUGIN_DATA) { $env:CLAUDE_PLUGIN_DATA } else { Join-Path $ClaudeDir "plugins\data\dsrcode" }
$BinDir = Join-Path $PluginData "bin"

# Runtime files use new dsrcode-* naming per DIST-29
$PidFile = Join-Path $ClaudeDir "dsrcode.pid"
$LogFile = Join-Path $ClaudeDir "dsrcode.log"
$RefcountFile = Join-Path $ClaudeDir "dsrcode.refcount"
$Binary = Join-Path $BinDir "dsrcode.exe"

# Lock-file for concurrent update protection per DIST-26
$LockFile = Join-Path $BinDir ".dsrcode-update.lock"

# ---- Ensure Directories ----
New-Item -ItemType Directory -Path $ClaudeDir -Force | Out-Null
New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

# ---- Session Tracking (refcount-based, PPID unreliable on Windows) ----
$CurrentCount = 0
if (Test-Path $RefcountFile) {
    $CurrentCount = [int](Get-Content $RefcountFile -ErrorAction SilentlyContinue)
}
$ActiveSessions = $CurrentCount + 1
$ActiveSessions | Out-File -FilePath $RefcountFile -Encoding ASCII -NoNewline

# ---- Check Running Daemon ----
if (Test-Path $PidFile) {
    $OldPid = Get-Content $PidFile -ErrorAction SilentlyContinue
    if ($OldPid) {
        $Process = Get-Process -Id $OldPid -ErrorAction SilentlyContinue
        if ($Process) {
            Write-Host "dsrcode already running (PID: $OldPid, sessions: $ActiveSessions)"
            exit 0
        }
    }
}

# ---- Lock-File Protection (DIST-26) ----
function Acquire-Lock {
    New-Item -ItemType Directory -Path (Split-Path $LockFile) -Force | Out-Null
    $lockPidFile = "$LockFile.pid"
    $attempts = 0
    while ((Test-Path $lockPidFile) -and $attempts -lt 30) {
        $lockPid = Get-Content $lockPidFile -ErrorAction SilentlyContinue
        if ($lockPid) {
            $lockProc = Get-Process -Id $lockPid -ErrorAction SilentlyContinue
            if (-not $lockProc) {
                Remove-Item $lockPidFile -Force -ErrorAction SilentlyContinue
                break
            }
        } else {
            Remove-Item $lockPidFile -Force -ErrorAction SilentlyContinue
            break
        }
        Start-Sleep -Seconds 1
        $attempts++
    }
    if ($attempts -ge 30) {
        Write-Host "Lock timeout" -ForegroundColor Yellow
        Remove-Item $lockPidFile -Force -ErrorAction SilentlyContinue
    }
    $PID | Out-File -FilePath $lockPidFile -Encoding ASCII -NoNewline
}

function Release-Lock {
    Remove-Item "$LockFile.pid" -Force -ErrorAction SilentlyContinue
}

# Rotate a log file when it exceeds 10 MB. Single-backup (.log.1, overwritten).
# Phase 7 D-10/D-11/D-12: prevent log truncation across daemon restarts.
# PowerShell Start-Process cannot append (upstream issue #15031), so we rotate
# BEFORE launch — the daemon then truncates the empty file and writes fresh.
function Rotate-Log {
    param([string]$LogPath)
    $maxSize = 10485760  # 10 MB
    if (-not (Test-Path $LogPath)) { return }
    $item = Get-Item $LogPath -ErrorAction SilentlyContinue
    if ($null -ne $item -and $item.Length -gt $maxSize) {
        Move-Item -Path $LogPath -Destination "$LogPath.1" -Force -ErrorAction SilentlyContinue
    }
}

# ---- Auto-patch settings.local.json (Phase 7 D-07: dual-register hooks) ----
# Mirrors scripts/start.sh patch_settings_local() for Windows-native users (no Git Bash invocation).
# Writes both the 13 HTTP hooks AND the SessionEnd command-hook fallback.
# Uses node.js via temp-file to avoid PowerShell here-string quote escaping.
function Patch-SettingsLocal {
    if (-not (Get-Command node -ErrorAction SilentlyContinue)) { return }
    $tempFile = Join-Path $env:TEMP "dsrcode-patch-settings-$PID.js"
    @'
const fs = require('fs');
const path = require('path');
const home = process.env.USERPROFILE || process.env.HOME;
const settingsPath = path.join(home, '.claude', 'settings.local.json');

let settings = {};
try { settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8')); } catch (e) { settings = {}; }
if (!settings.hooks || typeof settings.hooks !== 'object') { settings.hooks = {}; }

const DSRCODE_HOOKS = {
    'PreToolUse':         { matcher: '*',            slug: 'pre-tool-use' },
    'PostToolUse':        { matcher: '*',            slug: 'post-tool-use' },
    'PostToolUseFailure': { matcher: '*',            slug: 'post-tool-use-failure' },
    'UserPromptSubmit':   { matcher: null,           slug: 'user-prompt-submit' },
    'Stop':               { matcher: null,           slug: 'stop' },
    'StopFailure':        { matcher: '*',            slug: 'stop-failure' },
    'Notification':       { matcher: 'idle_prompt',  slug: 'notification' },
    'SubagentStart':      { matcher: '*',            slug: 'subagent-start' },
    'SubagentStop':       { matcher: '*',            slug: 'subagent-stop' },
    'PreCompact':         { matcher: '*',            slug: 'pre-compact' },
    'PostCompact':        { matcher: '*',            slug: 'post-compact' },
    'CwdChanged':         { matcher: null,           slug: 'cwd-changed' },
    'SessionEnd':         { matcher: null,           slug: 'session-end' }
};
const BASE_URL = 'http://127.0.0.1:19460/hooks/';
let added = 0;

for (const event of Object.keys(DSRCODE_HOOKS)) {
    const config = DSRCODE_HOOKS[event];
    if (!Array.isArray(settings.hooks[event])) settings.hooks[event] = [];
    const existing = settings.hooks[event];
    const hasDsrcode = existing.some(function(e) {
        return e && Array.isArray(e.hooks) && e.hooks.some(function(h) {
            return h && typeof h.url === 'string' && h.url.indexOf('127.0.0.1:19460') !== -1;
        });
    });
    if (!hasDsrcode) {
        const entry = { hooks: [{ type: 'http', url: BASE_URL + config.slug, timeout: 1 }] };
        if (config.matcher !== null) entry.matcher = config.matcher;
        existing.push(entry);
        added++;
    }
}

// Phase 7 D-07: SessionEnd command-hook fallback channel (Windows parity with Unix).
// Defends against upstream claude-code #17885/#33458/#35892 plugin SessionEnd unreliability.
const homeFwd = home.replace(/\\/g, '/');
const DSRCODE_COMMAND_HOOKS = {
    'SessionEnd': {
        command: 'bash -c \'ROOT="${CLAUDE_PLUGIN_ROOT:-' + homeFwd + '/.claude/plugins/marketplaces/dsrcode}"; bash "$ROOT/scripts/stop.sh"\'',
        timeout: 15
    }
};

for (const event of Object.keys(DSRCODE_COMMAND_HOOKS)) {
    const cfg = DSRCODE_COMMAND_HOOKS[event];
    if (!Array.isArray(settings.hooks[event])) settings.hooks[event] = [];
    const existingCmd = settings.hooks[event];
    const hasDsrcodeCmd = existingCmd.some(function(e) {
        return e && Array.isArray(e.hooks) && e.hooks.some(function(h) {
            return h && typeof h.command === 'string'
                && h.command.indexOf('dsrcode') !== -1
                && h.command.indexOf('stop.sh') !== -1;
        });
    });
    if (!hasDsrcodeCmd) {
        existingCmd.push({ hooks: [{ type: 'command', command: cfg.command, timeout: cfg.timeout }] });
        added++;
    }
}

try { fs.mkdirSync(path.dirname(settingsPath), { recursive: true }); } catch (e) {}
const tmp = settingsPath + '.tmp.' + process.pid;
fs.writeFileSync(tmp, JSON.stringify(settings, null, 2) + '\n');
fs.renameSync(tmp, settingsPath);

if (added > 0) console.log('settings.local.json patched: ' + added + ' dsrcode hook(s) added');
'@ | Set-Content -Path $tempFile -Encoding UTF8
    try {
        & node $tempFile 2>&1 | Out-Null
    } catch {
        # Silent failure — settings.local.json patch is best-effort.
    } finally {
        Remove-Item -Path $tempFile -Force -ErrorAction SilentlyContinue
    }
}

# ---- Download Function with SHA256 Verification (DIST-24) ----
function Download-Binary {
    $versionNoV = $Version.TrimStart("v")
    $archiveName = "dsrcode_${versionNoV}_windows_amd64.zip"
    $checksumsName = "dsrcode_${versionNoV}_checksums.txt"

    $downloadUrl = "https://github.com/$Repo/releases/download/$Version/$archiveName"
    $checksumsUrl = "https://github.com/$Repo/releases/download/$Version/$checksumsName"

    Write-Host "Downloading dsrcode $Version for windows-amd64..."

    $tmpDir = Join-Path $env:TEMP "dsrcode-download-$(Get-Random)"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    try {
        # Download archive
        try {
            Invoke-WebRequest -Uri $downloadUrl -OutFile (Join-Path $tmpDir "archive.zip") -UseBasicParsing -ErrorAction Stop
        } catch {
            Write-Host "Warning: Download failed." -ForegroundColor Yellow
            Write-Host "If behind a proxy, set HTTP_PROXY/HTTPS_PROXY." -ForegroundColor Yellow
            return $false
        }

        # Verify size (HTML error pages are <10KB, real archives are >1MB)
        $archivePath = Join-Path $tmpDir "archive.zip"
        $fileSize = (Get-Item $archivePath).Length
        if ($fileSize -lt 100000) {
            Write-Host "Warning: Downloaded file too small ($fileSize bytes)" -ForegroundColor Yellow
            return $false
        }

        # Download and verify SHA256 checksum (Get-FileHash is built-in)
        try {
            $checksumsPath = Join-Path $tmpDir "checksums.txt"
            Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing -ErrorAction Stop

            $checksumLines = Get-Content $checksumsPath
            $expectedLine = $checksumLines | Where-Object { $_ -match $archiveName }
            if ($expectedLine) {
                $expectedHash = ($expectedLine -split '\s+')[0]
                $actualHash = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()
                if ($expectedHash -and ($actualHash -ne $expectedHash)) {
                    Write-Host "ERROR: SHA256 checksum mismatch!" -ForegroundColor Red
                    Write-Host "  Expected: $expectedHash" -ForegroundColor Red
                    Write-Host "  Got:      $actualHash" -ForegroundColor Red
                    return $false
                }
            }
        } catch {
            # Checksum download failed -- continue without verification
        }

        # Extract binary from archive (Expand-Archive is built-in per DIST-24)
        $extractDir = Join-Path $tmpDir "extract"
        Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

        $extractedBinary = Join-Path $extractDir "dsrcode.exe"
        if (-not (Test-Path $extractedBinary)) {
            Write-Host "Warning: dsrcode.exe not found in archive" -ForegroundColor Yellow
            return $false
        }

        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
        Copy-Item $extractedBinary $Binary -Force

        Write-Host "Downloaded and verified successfully!"
        return $true
    } finally {
        Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# ---- Build-from-Source Fallback ----
function Find-SourceDir {
    $pluginRoot = if ($env:CLAUDE_PLUGIN_ROOT) { $env:CLAUDE_PLUGIN_ROOT } else { Join-Path $ClaudeDir "plugins\marketplaces\dsrcode" }
    $candidates = @($pluginRoot, (Join-Path $env:USERPROFILE "Projects\cc-discord-presence"), (Join-Path $env:USERPROFILE "Projects\dsrcode"))
    foreach ($candidate in $candidates) {
        if (Test-Path (Join-Path $candidate "go.mod")) {
            return $candidate
        }
    }
    return $null
}

function Build-FromSource {
    $sourceDir = Find-SourceDir
    if (-not $sourceDir) {
        return $false
    }

    $goCmd = Get-Command "go" -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        return $false
    }

    Write-Host "Building dsrcode from source ($sourceDir)..."
    $versionNoV = $Version.TrimStart("v")
    # Use lowercase version per DIST-06 for GoReleaser compat
    $ldflags = "-X main.version=$versionNoV"

    Push-Location $sourceDir
    try {
        & go build -ldflags $ldflags -o $Binary . 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Built successfully!"
            return $true
        } else {
            Write-Host "Warning: Build failed" -ForegroundColor Yellow
            return $false
        }
    } finally {
        Pop-Location
    }
}

# ---- Install Help Message (DIST-27) ----
function Show-InstallHelp {
    Write-Host ""
    Write-Host "==== dsrcode: Installation Failed ====" -ForegroundColor Red
    Write-Host ""
    Write-Host "Could not download the binary and could not build from source."
    Write-Host ""
    Write-Host "Option 1: Download manually"
    Write-Host "  https://github.com/$Repo/releases/tag/$Version"
    Write-Host "  Place the binary in: $BinDir\"
    Write-Host ""
    Write-Host "Option 2: Install Go (https://go.dev/dl/) and restart Claude Code"
    Write-Host ""
    Write-Host "If behind a proxy, set HTTP_PROXY/HTTPS_PROXY."
    Write-Host "========================================="
}

# ---- Acquire Binary: Download First, Build Fallback, Error Last ----
function Ensure-Binary {
    Acquire-Lock
    # Re-check after acquiring lock (another process may have installed)
    if (Test-Path $Binary) {
        Release-Lock
        return $true
    }
    if (Download-Binary) {
        Release-Lock
        return $true
    }
    Write-Host "Trying build from source as fallback..."
    if (Build-FromSource) {
        Release-Lock
        return $true
    }
    Release-Lock
    Show-InstallHelp
    return $false
}

# ---- Kill Running Daemon ----
function Kill-DaemonIfRunning {
    if (Test-Path $PidFile) {
        $oldPid = Get-Content $PidFile -ErrorAction SilentlyContinue
        if ($oldPid) {
            $proc = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
            if ($proc) {
                Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue
                Start-Sleep -Seconds 1
            }
        }
        Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
    }
    # Also check old PID file path for migration scenarios
    $oldPidFile = Join-Path $ClaudeDir "discord-presence.pid"
    if (Test-Path $oldPidFile) {
        $oldPid = Get-Content $oldPidFile -ErrorAction SilentlyContinue
        if ($oldPid) {
            $proc = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
            if ($proc) {
                Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue
                Start-Sleep -Seconds 1
            }
        }
        Remove-Item $oldPidFile -Force -ErrorAction SilentlyContinue
    }
}

# ---- Old Binary Migration (DIST-37) ----
function Migrate-OldBinary {
    $oldBinDir = Join-Path $env:USERPROFILE ".claude\bin"
    $oldBinary = Join-Path $oldBinDir "cc-discord-presence-windows-amd64.exe"

    if (Test-Path $oldBinary) {
        Write-Host "Migrating old binary: cc-discord-presence-windows-amd64.exe -> $BinDir\dsrcode.exe"
        Kill-DaemonIfRunning

        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
        Copy-Item $oldBinary $Binary -Force
        Remove-Item $oldBinary -Force -ErrorAction SilentlyContinue

        Write-Host "Migration complete."
        return $true
    }
    return $false
}

# ---- Version Check + Binary Acquisition (DIST-21, DIST-22) ----
if (-not (Test-Path $Binary)) {
    # Try migrating old binary first
    if (-not (Migrate-OldBinary)) {
        # No old binary -- acquire fresh
        if (-not (Ensure-Binary)) {
            exit 1
        }
    }
} else {
    # Binary exists -- check version
    $currentVersion = "unknown"
    try {
        $versionOutput = & $Binary --version 2>$null
        if ($versionOutput -match 'dsrcode\s+(\S+)') {
            $currentVersion = $Matches[1]
        }
    } catch {}

    $currentNormalized = $currentVersion.TrimStart("v")
    $expectedNormalized = $Version.TrimStart("v")

    # Skip update check for dev/unknown versions per DIST-22
    if ($currentNormalized -eq "dev" -or $currentNormalized -eq "unknown") {
        Write-Host "dsrcode running dev/local build, skipping version check"
    } elseif ($currentNormalized -ne "" -and $currentNormalized -ne $expectedNormalized) {
        Write-Host "Updating dsrcode from $currentVersion to $Version..."
        # Must kill daemon before replacing binary (Windows locks running .exe)
        Kill-DaemonIfRunning
        Remove-Item $Binary -Force -ErrorAction SilentlyContinue
        Acquire-Lock
        if (-not (Download-Binary)) {
            Write-Host "Trying build from source as fallback..."
            if (-not (Build-FromSource)) {
                Release-Lock
                Show-InstallHelp
                exit 1
            }
        }
        Release-Lock
    }
}

# Final guard
if (-not (Test-Path $Binary)) {
    Write-Host "Error: Binary not found at $Binary" -ForegroundColor Red
    exit 1
}

# From this point on, stop on errors
$ErrorActionPreference = "Stop"

# Phase 7 D-10/D-11/D-12: rotate logs at 10 MB before launching daemon
$LogFileErr = "$LogFile.err"
Rotate-Log $LogFile
Rotate-Log $LogFileErr

# ---- Auto-patch settings.local.json with dsrcode hooks (Phase 7 D-07) ----
Patch-SettingsLocal

# ---- Start Daemon (hidden window, PID capture) ----
$Process = Start-Process -FilePath $Binary -WindowStyle Hidden -PassThru -RedirectStandardOutput $LogFile -RedirectStandardError $LogFileErr
$Process.Id | Out-File -FilePath $PidFile -Encoding ASCII -NoNewline

# ---- Health Check on port 19460 (unchanged per DIST-40) ----
$healthOk = $false
for ($i = 0; $i -lt 50; $i++) {
    try {
        $response = Invoke-WebRequest -Uri "http://127.0.0.1:19460/health" -UseBasicParsing -TimeoutSec 1 -ErrorAction SilentlyContinue
        if ($response.StatusCode -eq 200) {
            $healthOk = $true
            break
        }
    } catch {}
    Start-Sleep -Milliseconds 100
}

if ($healthOk) {
    Write-Host "dsrcode started (PID: $($Process.Id), sessions: $ActiveSessions)"
} else {
    $daemonProc = Get-Process -Id $Process.Id -ErrorAction SilentlyContinue
    if ($daemonProc) {
        Write-Host "WARNING: dsrcode started (PID: $($Process.Id)) but health check timed out" -ForegroundColor Yellow
    } else {
        Write-Host "ERROR: dsrcode failed to start (port 19460 may be in use). Check: ~/.claude/dsrcode.log" -ForegroundColor Red
        Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
    }
}

# ---- First-Run Hint ----
$ConfigFile = Join-Path $ClaudeDir "dsrcode-config.json"
if (-not (Test-Path $ConfigFile)) {
    Write-Host "dsrcode started (Preset: minimal) -- /dsrcode:setup for customization"
}

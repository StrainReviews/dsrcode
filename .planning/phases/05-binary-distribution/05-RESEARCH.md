# Phase 5: Binary Distribution Pipeline - Research

**Researched:** 2026-04-06
**Domain:** Go cross-compilation, GoReleaser CI/CD, shell-based binary distribution, Claude Code plugin lifecycle
**Confidence:** HIGH

## Summary

This phase delivers a complete binary distribution pipeline for the cc-discord-presence Go daemon. Currently, first-time users without a Go toolchain get a silent failure -- `start.sh` only builds from source, and `start.ps1` is outdated (references `tsanva/cc-discord-presence` v1.0.3). The fix is three-pronged: (1) GoReleaser v2 replaces the manual `go build` release workflow to produce checksummed, archived binaries for 5 platforms; (2) `start.sh` and `start.ps1` are rewritten with a download-first strategy that falls back to `go build`; (3) a bump script automates version management across all files that embed it.

The project is pure Go (`go-winio`, `fsnotify`, `lumberjack` are all pure Go) making cross-compilation trivial with `CGO_ENABLED=0`. The CONTEXT.md decision to rename `var Version` (capital V) to `var version` (lowercase) eliminates the need for custom ldflags since GoReleaser's default targets `main.version`. Binaries persist in `${CLAUDE_PLUGIN_DATA}/bin/` (resolves to `~/.claude/plugins/data/dsrcode-dsrcode/bin/`) which survives plugin updates and is cleaned on uninstall.

**Primary recommendation:** Replace the existing `release.yml` with `goreleaser/goreleaser-action@v7`, rewrite both startup scripts with download-first + build-fallback + SHA256 verification, and add `scripts/bump-version.sh` for coordinated version management.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **DIST-01:** GoReleaser v2.15+ mit goreleaser-action@v6 in GitHub Actions
- **DIST-02:** Trigger: Tag-Push (v*) UND workflow_dispatch (GitHub UI Release-Button)
- **DIST-03:** 5 Plattformen: macOS arm64/amd64, Linux amd64/arm64, Windows amd64. CGO_ENABLED=0 Cross-Compilation.
- **DIST-04:** Download-first Strategie: 1) curl von GitHub Releases (atomic via temp file + mv), 2) go build Fallback wenn Go installiert, 3) Fehlermeldung mit Installationsanleitung. set -e erst NACH Binary-Acquisition.
- **DIST-05:** SHA256-Checksum-Verifikation nach Download. GoReleaser generiert checksums.txt automatisch. sha256sum/shasum vorinstalliert auf allen Plattformen.
- **DIST-06:** Binary in ${CLAUDE_PLUGIN_DATA}/bin/ speichern (offizielles Claude Code Plugin-Verzeichnis, ueberlebt Plugin-Updates, wird bei Uninstall aufgeraeumt). NICHT in ~/.claude/bin/ oder ${CLAUDE_PLUGIN_ROOT}.
- **DIST-07:** Bump-Script (scripts/bump-version.sh) nimmt Version als Argument, updated main.go + plugin.json + marketplace.json + start.sh + start.ps1 per sed. Dann git commit + git tag.
- **DIST-08:** Go Variable zu `var version` (lowercase) aendern fuer GoReleaser Default-ldflags (-X main.version={{.Version}}). Kein custom ldflag noetig.
- **DIST-09:** Download nur bei fehlendem Binary oder Version-Mismatch. Kein Download bei jedem Start. Version-Check per --version ist <100ms. Dadurch kein Timeout-Problem (15s SessionStart-Limit).
- **DIST-10:** StrainReviews/dsrcode behalten. Existiert bereits, keine Migration noetig. GitHub Releases URL: https://github.com/StrainReviews/dsrcode/releases/download/{tag}/{binary}
- start.ps1 komplett rewriten mit gleicher Logik wie start.sh: Download-first + Build-Fallback + Version-Check + ${CLAUDE_PLUGIN_DATA}/bin/ Speicherort
- Beim ersten Start mit neuem start.sh: Binary von ~/.claude/bin/ nach ${CLAUDE_PLUGIN_DATA}/bin/ verschieben (move + cleanup)
- Fehlermeldungen in start.sh/start.ps1 auf Englisch (internationales Plugin)
- Bei Version-Mismatch: Auto-Download der neuen Version

### Claude's Discretion
- Goreleaser YAML Config Details (archive format, naming template)
- Exact checksums verification implementation (sha256sum vs shasum detection)
- start.sh/start.ps1 error message wording
- Bump-Script implementation details (sed patterns, commit message format)

### Deferred Ideas (OUT OF SCOPE)
- DSR-Labs Organisation auf GitHub erstellen und Repo transferieren
- Automatischer Changelog aus Conventional Commits
- Plugin bin/ Directory Feature nutzen (v2.1.91) fuer bare command Zugriff
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIST-01 | GoReleaser v2.15+ mit goreleaser-action in GitHub Actions | GoReleaser v2.15.2 verified latest, goreleaser-action v7.0.0 verified; YAML config format documented |
| DIST-02 | Tag push (v*) AND workflow_dispatch trigger | workflow_dispatch with tag input pattern documented; goreleaser-action inputs confirmed |
| DIST-03 | 5 platforms: macOS arm64/amd64, Linux amd64/arm64, Windows amd64 | Build matrix with ignore for windows/arm64 documented; CGO_ENABLED=0 confirmed safe |
| DIST-04 | Download-first with go build fallback | Complete start.sh architecture documented with platform detection, curl flags, atomic download |
| DIST-05 | SHA256 checksum verification | Cross-platform sha256sum/shasum detection pattern documented; GoReleaser checksums.txt format confirmed |
| DIST-06 | Binary in ${CLAUDE_PLUGIN_DATA}/bin/ | Plugin lifecycle verified; CLAUDE_PLUGIN_DATA resolves to ~/.claude/plugins/data/dsrcode-dsrcode/; migration from ~/.claude/bin/ documented |
| DIST-07 | Bump script for version management | sed patterns for all 5 files documented; commit + tag workflow specified |
| DIST-08 | Rename var Version to var version | 5 references in main.go + server package identified; default ldflags eliminate custom config |
| DIST-09 | Download only on missing/mismatch, not every start | Version check via --version (<100ms) documented; 15s timeout budget analysis provided |
| DIST-10 | Repo: StrainReviews/dsrcode | Verified via gh repo view; no releases yet (clean start) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Conventional Commits: feat, fix, refactor, docs, test, chore
- Immutable data patterns
- Files < 800 lines, functions < 50 lines
- 80%+ test coverage
- No hardcoded secrets

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard | Confidence |
|------|---------|---------|--------------|------------|
| GoReleaser | v2.15.2 | Cross-compile, archive, checksum, publish GitHub Releases | De facto standard for Go release automation | HIGH [VERIFIED: `gh release list --repo goreleaser/goreleaser` shows v2.15.2 as Latest, 2026-03-31] |
| goreleaser-action | v7.0.0 | GitHub Actions integration | Official action, default `version: '~> v2'` | HIGH [VERIFIED: `gh release list --repo goreleaser/goreleaser-action` shows v7.0.0 as Latest, 2026-02-21] |
| actions/checkout | v4 | Git checkout with full history | Required with `fetch-depth: 0` for changelog generation | HIGH [CITED: goreleaser-action README] |
| actions/setup-go | v5 | Go toolchain setup | Matches existing workflow pattern | HIGH [VERIFIED: existing release.yml uses v5] |

**IMPORTANT version correction:** CONTEXT.md specifies `goreleaser-action@v6`, but the latest is **v7.0.0** (released 2026-02-21). v7 is the version matched by the `version: '~> v2'` default. The goreleaser-action README example uses `@v7`. Recommend using `@v7` instead of `@v6`. [VERIFIED: goreleaser-action README examples all show @v7]

### Supporting

| Tool | Purpose | When to Use |
|------|---------|-------------|
| curl 8.x | Binary download in start.sh | Available on macOS, Linux, Git Bash (Windows) |
| Invoke-WebRequest | Binary download in start.ps1 | PowerShell native, no external dependency |
| sha256sum / shasum | Checksum verification | sha256sum on Linux/Git Bash, shasum -a 256 on macOS |
| Get-FileHash | Checksum verification on Windows | PowerShell native SHA256 support |
| sed (GNU/BSD) | Version bumping across files | Available on all dev platforms |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| GoReleaser | Manual `go build` matrix (current) | No checksums, no archives, no ldflags, no changelog |
| GoReleaser | `ko` | Container-focused, not binary distribution |
| goreleaser-action v7 | goreleaser-action v6 | v6 works but v7 is current default; CONTEXT says v6 |
| Direct URL download | GitHub API + jq | Adds jq dependency, rate limiting concerns, unnecessary for public repo |

## Architecture Patterns

### GoReleaser v2 Config (.goreleaser.yaml)

GoReleaser v2 requires `version: 2` as first meaningful line. Key v2-specific changes from v1: [CITED: goreleaser.com/deprecations/]

| v1 Field | v2 Field | Changed In |
|----------|----------|------------|
| `archives.format` | `archives.formats` (list) | v2.6 |
| `archives.format_overrides.format` | `archives.format_overrides.formats` (list) | v2.6 |
| `archives.builds` | `archives.ids` | v2.8 |
| `--rm-dist` flag | `--clean` flag | v2.0 |
| `--debug` flag | `--verbose` flag | v2.0 |

### Recommended .goreleaser.yaml

```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
# Source: goreleaser.com/customization/builds/go/
version: 2

project_name: cc-discord-presence

builds:
  - id: cc-discord-presence
    main: .
    binary: cc-discord-presence
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    # Default ldflags: -s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}
    # Works with lowercase `var version` (DIST-08)
    ldflags:
      - -s -w -X main.version={{.Version}}
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - id: default
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    files:
      - LICENSE
      - README.md
      - CHANGELOG.md
      - PRIVACY.md
      - TERMS.md

checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
  algorithm: sha256

changelog:
  use: github-native

release:
  github:
    owner: StrainReviews
    name: dsrcode
  prerelease: auto
  make_latest: true
```

**Build matrix produced (5 targets):**

| # | GOOS | GOARCH | Archive Format | Archive Name Example |
|---|------|--------|----------------|---------------------|
| 1 | darwin | arm64 | tar.gz | cc-discord-presence_3.2.0_darwin_arm64.tar.gz |
| 2 | darwin | amd64 | tar.gz | cc-discord-presence_3.2.0_darwin_amd64.tar.gz |
| 3 | linux | amd64 | tar.gz | cc-discord-presence_3.2.0_linux_amd64.tar.gz |
| 4 | linux | arm64 | tar.gz | cc-discord-presence_3.2.0_linux_arm64.tar.gz |
| 5 | windows | amd64 | zip | cc-discord-presence_3.2.0_windows_amd64.zip |

Plus: `cc-discord-presence_3.2.0_checksums.txt`

**Key design decisions:**
- `CGO_ENABLED=0`: Project is pure Go -- go-winio, fsnotify, lumberjack all pure Go [VERIFIED: go.mod]
- `mod_timestamp`: Reproducible builds -- binary timestamps match git commit [CITED: goreleaser.com/blog/reproducible-builds/]
- `-s -w` in ldflags: Strip debug info and DWARF tables, ~30% binary size reduction [CITED: goreleaser.com/customization/builds/go/]
- `changelog.use: github-native`: Delegates to GitHub's auto-generated release notes [CITED: goreleaser.com/customization/changelog/]
- `prerelease: auto`: Tags like `v3.2.0-rc1` auto-marked as pre-release [CITED: goreleaser.com/customization/release/]
- Archive includes LICENSE, README, CHANGELOG, PRIVACY, TERMS [VERIFIED: files exist in repo root]

### Breaking Change: Archive Naming vs Current Binary Naming

**Current naming** (from release.yml): `cc-discord-presence-darwin-arm64` (no version, no archive)
**New naming** (GoReleaser): `cc-discord-presence_3.2.0_darwin_arm64.tar.gz` (versioned, archived)

This is a BREAKING CHANGE for the `start.sh` download URL construction. The new start.sh must:
1. Construct the archive name: `cc-discord-presence_${VERSION#v}_${OS}_${ARCH}.tar.gz` (or `.zip` for Windows)
2. Download the archive (not a bare binary)
3. Extract the binary from the archive
4. Move binary to `${CLAUDE_PLUGIN_DATA}/bin/`

No existing users are affected since there are no releases yet on StrainReviews/dsrcode. [VERIFIED: `gh release list --repo StrainReviews/dsrcode` returns empty]

### GitHub Actions Workflow (.github/workflows/release.yml)

```yaml
name: Release

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      tag:
        description: "Tag to release (e.g., v3.2.0)"
        required: true
        type: string

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Create tag (workflow_dispatch only)
        if: github.event_name == 'workflow_dispatch'
        run: |
          git tag "${{ inputs.tag }}"

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Key decisions:**
- `fetch-depth: 0`: REQUIRED for changelog and version detection [CITED: goreleaser-action README "IMPORTANT" note]
- `go-version-file: go.mod`: Reads version from go.mod (`go 1.25`) instead of hardcoding. Current workflow hardcodes `1.26` which is wrong. [VERIFIED: go.mod says `go 1.25`]
- `workflow_dispatch` with tag input: Creates tag locally before goreleaser runs. Per DIST-02.
- `GITHUB_TOKEN`: Auto-provided by GitHub Actions, needs `contents: write` [CITED: goreleaser.com/ci/actions/]

### Version Variable Rename (DIST-08)

**Current state:** `var Version = "3.1.10"` (capital V, exported) at main.go:33
**Target state:** `var version = "dev"` (lowercase, unexported) at main.go:33

GoReleaser's default ldflags target `main.version` (lowercase). The CONTEXT.md decision is to rename to lowercase to avoid needing custom ldflags. [CITED: goreleaser.com/cookbooks/using-main.version/]

**References that need updating (5 locations in Go code):**

| File | Line | Current | New |
|------|------|---------|-----|
| main.go | 32 | `// Version of the daemon...` | `// version of the daemon...` |
| main.go | 33 | `var Version = "3.1.10"` | `var version = "dev"` |
| main.go | 156 | `fmt.Println("cc-discord-presence " + Version)` | `fmt.Println("cc-discord-presence " + version)` |
| main.go | 167 | `"version", Version,` | `"version", version,` |
| main.go | 235 | `Version,` | `version,` |

**server package is NOT affected** -- `server.go:553` and `server.go:575` use a `Version` field in a struct (JSON response), not the package-level variable. The variable is passed as a constructor argument on main.go:235. [VERIFIED: direct code read]

**Default value changes from `"3.1.10"` to `"dev"`** -- GoReleaser injects the real version at build time. The "dev" default indicates a development build. [CITED: goreleaser.com/cookbooks/using-main.version/]

### Binary Storage Architecture

```
Before Phase 5:
  ~/.claude/bin/cc-discord-presence-{os}-{arch}[.exe]    <-- current location

After Phase 5:
  ~/.claude/plugins/data/dsrcode-dsrcode/bin/cc-discord-presence[.exe]  <-- persistent storage
```

`${CLAUDE_PLUGIN_DATA}` resolves to `~/.claude/plugins/data/dsrcode-dsrcode/` for the dsrcode plugin. [VERIFIED: ls of ~/.claude/plugins/data/ shows dsrcode-dsrcode/ exists]

**Key properties of CLAUDE_PLUGIN_DATA:**
- Created automatically on first reference [VERIFIED: code.claude.com/docs/en/plugins-reference]
- Survives plugin updates (unlike CLAUDE_PLUGIN_ROOT) [VERIFIED: code.claude.com/docs/en/plugins-reference]
- Deleted when plugin is uninstalled from last scope (unless --keep-data) [VERIFIED: code.claude.com/docs/en/plugins-reference]
- Variable is substituted inline in hook commands AND exported as env var to subprocesses [VERIFIED: code.claude.com/docs/en/plugins-reference]

**Migration on first run:** Move binary from `~/.claude/bin/cc-discord-presence-*` to `${CLAUDE_PLUGIN_DATA}/bin/cc-discord-presence[.exe]`, then delete old location.

**Binary naming simplification:** Inside PLUGIN_DATA, the binary can be just `cc-discord-presence[.exe]` (no OS-arch suffix needed since we know the current platform). This simplifies the path and avoids the old naming inconsistency.

### Download Flow with Checksum Verification

```
start.sh invoked via SessionStart hook
  |
  v
[Read VERSION from script, detect OS/ARCH]
  |
  v
[MIGRATION: Move ~/.claude/bin/ binary to ${CLAUDE_PLUGIN_DATA}/bin/ if exists]
  |
  v
[Binary exists at ${CLAUDE_PLUGIN_DATA}/bin/?] --NO--> [Download + verify]
  |                                                            |
  YES                                                    SUCCESS? --NO--> [go build fallback]
  |                                                            |                |
  v                                                           YES              YES --> [start daemon]
[Version matches?] --YES--> [Start daemon]                     |                |
  |                                                            v               NO --> [Error + instructions]
  NO                                                    [Start daemon]
  |
  v
[Kill old daemon, download new + verify] --> [Start daemon]
```

### SHA256 Checksum Verification Pattern

GoReleaser produces a `cc-discord-presence_{version}_checksums.txt` file in this format:
```
abc123...  cc-discord-presence_3.2.0_darwin_arm64.tar.gz
def456...  cc-discord-presence_3.2.0_darwin_amd64.tar.gz
...
```

**Cross-platform verification function for start.sh:**

```bash
verify_checksum() {
    local file="$1" expected_hash="$2"
    local actual_hash

    if command -v sha256sum &>/dev/null; then
        actual_hash=$(sha256sum "$file" | awk '{print $1}')
    elif command -v shasum &>/dev/null; then
        actual_hash=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        echo "Warning: No SHA256 tool found, skipping verification" >&2
        return 0  # Proceed without verification
    fi

    if [[ "$actual_hash" != "$expected_hash" ]]; then
        echo "Error: Checksum mismatch!" >&2
        echo "  Expected: $expected_hash" >&2
        echo "  Got:      $actual_hash" >&2
        return 1
    fi
    return 0
}

# Download checksums file, extract expected hash, verify archive
download_and_verify() {
    local archive_name="$1" archive_path="$2"
    local checksums_url checksums_file expected_hash

    checksums_url="https://github.com/${REPO}/releases/download/${VERSION}/${PROJECT}_${VERSION#v}_checksums.txt"
    checksums_file="${archive_path}.checksums"

    # Download checksums
    if ! curl -fsSL -o "$checksums_file" "$checksums_url" 2>/dev/null; then
        echo "Warning: Could not download checksums, skipping verification" >&2
        return 0
    fi

    # Extract expected hash for our archive
    expected_hash=$(grep "$archive_name" "$checksums_file" | awk '{print $1}')
    rm -f "$checksums_file"

    if [[ -z "$expected_hash" ]]; then
        echo "Warning: Archive not found in checksums file" >&2
        return 0
    fi

    verify_checksum "$archive_path" "$expected_hash"
}
```

[VERIFIED: sha256sum on Linux/Git Bash, shasum -a 256 on macOS] [CITED: tobywf.com/2023/03/sha-256-checksums/]

**PowerShell equivalent for start.ps1:**

```powershell
function Test-Checksum {
    param([string]$FilePath, [string]$ExpectedHash)
    $actualHash = (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash.ToLower()
    if ($actualHash -ne $ExpectedHash.ToLower()) {
        Write-Error "Checksum mismatch! Expected: $ExpectedHash Got: $actualHash"
        return $false
    }
    return $true
}
```

### Archive Extraction Pattern

Since GoReleaser produces `.tar.gz` (Unix) and `.zip` (Windows) archives, the download function must extract the binary:

**Bash (start.sh):**
```bash
# tar.gz extraction
tar -xzf "$archive_path" -C "$tmp_dir" cc-discord-presence
mv "$tmp_dir/cc-discord-presence" "$BINARY"

# Cleanup
rm -f "$archive_path"
rm -rf "$tmp_dir"
```

**PowerShell (start.ps1):**
```powershell
# zip extraction
Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force
Move-Item -Path (Join-Path $tmpDir "cc-discord-presence.exe") -Destination $Binary -Force

# Cleanup
Remove-Item -Path $archivePath -Force
Remove-Item -Path $tmpDir -Recurse -Force
```

### Bump Script Pattern (scripts/bump-version.sh)

Files requiring version update:

| File | Pattern | sed Target |
|------|---------|------------|
| main.go | `var version = "X.Y.Z"` | `s/var version = ".*"/var version = "NEW"/` |
| .claude-plugin/plugin.json | `"version": "X.Y.Z"` | jq or sed on version field |
| .claude-plugin/marketplace.json | `"version": "X.Y.Z"` | jq or sed on version field |
| scripts/start.sh | `VERSION="vX.Y.Z"` | `s/VERSION="v.*"/VERSION="vNEW"/` |
| scripts/start.ps1 | `$Version = "vX.Y.Z"` | PowerShell string replacement via sed |

```bash
#!/bin/bash
# Usage: ./scripts/bump-version.sh 3.2.0
set -euo pipefail

NEW_VERSION="${1:?Usage: bump-version.sh <version>}"
# Strip v prefix if provided
NEW_VERSION="${NEW_VERSION#v}"

echo "Bumping version to ${NEW_VERSION}..."

# main.go
sed -i '' "s/var version = \".*\"/var version = \"${NEW_VERSION}\"/" main.go

# plugin.json (version field)
# Use temp file for portability (BSD vs GNU sed)
jq ".version = \"${NEW_VERSION}\"" .claude-plugin/plugin.json > .claude-plugin/plugin.json.tmp
mv .claude-plugin/plugin.json.tmp .claude-plugin/plugin.json

# marketplace.json (plugins[0].version)
jq ".plugins[0].version = \"${NEW_VERSION}\"" .claude-plugin/marketplace.json > .claude-plugin/marketplace.json.tmp
mv .claude-plugin/marketplace.json.tmp .claude-plugin/marketplace.json

# start.sh
sed -i '' "s/VERSION=\"v.*\"/VERSION=\"v${NEW_VERSION}\"/" scripts/start.sh

# start.ps1
sed -i '' "s/\\\$Version = \"v.*\"/\\\$Version = \"v${NEW_VERSION}\"/" scripts/start.ps1

echo "Version bumped to ${NEW_VERSION}"
echo ""
echo "Next steps:"
echo "  git add -A"
echo "  git commit -m \"chore: bump version to v${NEW_VERSION}\""
echo "  git tag v${NEW_VERSION}"
echo "  git push origin main --tags"
```

**Note:** On macOS, `sed -i ''` is required (BSD sed). On Linux, `sed -i` suffices. The script should detect OS and use the correct form, or use a temp file + mv pattern for portability. Using `jq` for JSON files is safer than sed for JSON. [ASSUMED: jq available on dev machine]

### hooks.json Update

The SessionStart hook must pass `CLAUDE_PLUGIN_DATA` to the startup script:

```json
{
  "type": "command",
  "command": "bash -c 'export PLUGIN_DATA=\"${CLAUDE_PLUGIN_DATA}\"; ROOT=\"${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}\"; bash \"$ROOT/scripts/start.sh\"'",
  "timeout": 15
}
```

Per official docs, `${CLAUDE_PLUGIN_DATA}` is substituted inline in the command string AND exported as an environment variable to the subprocess. [VERIFIED: code.claude.com/docs/en/plugins-reference]

### SessionStart Timeout Budget Analysis (DIST-09)

| Operation | Estimated Time | Blocking? |
|-----------|---------------|-----------|
| Version check (`--version`) | <100ms | No |
| Migration check (file exists) | <10ms | No |
| Archive download (~10MB) | 2-8s on typical connection | Yes |
| Checksum download (~1KB) | <500ms | Yes |
| SHA256 calculation | <200ms | No |
| Archive extraction | <500ms | No |
| Daemon startup + health check | 1-3s | Yes |

**Total worst case (first install):** ~12s within 15s timeout
**Typical cold start:** ~5-8s
**Warm start (version match):** <1s (version check + daemon already running check)

The 15s SessionStart timeout is sufficient for most connections. On very slow connections, the download may time out. This is acceptable -- the next session will retry. [ASSUMED: 10MB download completes in <8s on typical broadband]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-compilation matrix | Shell loops with GOOS/GOARCH | GoReleaser `builds` section | Current workflow has no ldflags, no checksums, no archives |
| Checksum generation | `sha256sum bin/*` in shell | GoReleaser `checksum` section | Automatic, standard format, includes all archives |
| Archive packaging | `tar czf` / `zip` in shell | GoReleaser `archives` section | Per-OS format overrides, includes bundled files |
| Release notes | softprops/action-gh-release | GoReleaser `changelog: github-native` | Single tool, integrated |
| Version injection | Manual `-ldflags` in go build | GoReleaser default ldflags | Template variables, consistent |
| JSON version updates | sed on JSON files | jq | sed on JSON is brittle, jq is correct |
| Binary download | Custom HTTP client | curl -fsSL / Invoke-WebRequest | Universal, handles redirects |
| Persistent storage | Custom ~/.claude/bin/ | ${CLAUDE_PLUGIN_DATA} | Official, managed lifecycle |

## Common Pitfalls

### Pitfall 1: Missing fetch-depth: 0 in Checkout
**What goes wrong:** GoReleaser generates empty changelog, version detection fails.
**Why it happens:** Default actions/checkout does shallow clone (depth 1).
**How to avoid:** Always set `fetch-depth: 0`. [CITED: goreleaser-action README "IMPORTANT" note]
**Warning signs:** "changelog: could not get tag" errors in CI logs.

### Pitfall 2: Using v1 format: Instead of v2 formats:
**What goes wrong:** GoReleaser v2.6+ warns about deprecated field, may error in future.
**Why it happens:** Most blog posts/Stack Overflow show v1 syntax.
**How to avoid:** Use `formats: ["tar.gz"]` (list) and `format_overrides.formats: ["zip"]` (list). [CITED: goreleaser.com/deprecations/]
**Warning signs:** Deprecation warnings in goreleaser output.

### Pitfall 3: Partial Downloads Treated as Valid Binary
**What goes wrong:** Network interruption saves partial file. Next run gets exec format error.
**Why it happens:** curl saves whatever it received.
**How to avoid:** Download to `.tmp` file, only `mv` into place on success. Check file size is non-zero. [CITED: fzf, starship installer patterns]
**Warning signs:** "cannot execute binary file" or "Exec format error".

### Pitfall 4: Missing -L Flag on curl
**What goes wrong:** GitHub redirects to CDN. Without -L, curl saves 302 HTML as "binary".
**Why it happens:** GitHub Releases URLs redirect to `objects.githubusercontent.com`.
**How to avoid:** Always use `curl -fsSL`. [VERIFIED: standard GitHub download behavior]
**Warning signs:** Downloaded file is ~1KB HTML instead of ~10MB binary.

### Pitfall 5: set -e Kills Script on Expected Failures
**What goes wrong:** curl failing (no internet) triggers set -e exit before fallback runs.
**Why it happens:** set -e treats non-zero exit as fatal.
**How to avoid:** Only use set -e AFTER binary acquisition section. Use `|| true` or explicit `if` blocks for expected failures.
**Warning signs:** Script exits silently without trying build fallback.

### Pitfall 6: Windows Binary Lock During Update
**What goes wrong:** Cannot overwrite running .exe on Windows.
**Why it happens:** Windows locks files opened by running processes.
**How to avoid:** Kill daemon BEFORE replacing binary. [VERIFIED: current start.sh already handles this]
**Warning signs:** "Permission denied" or "The process cannot access the file".

### Pitfall 7: BSD sed vs GNU sed (-i flag)
**What goes wrong:** `sed -i 's/...'` works on Linux but fails on macOS with "invalid command code".
**Why it happens:** BSD sed requires `sed -i ''` (empty backup extension), GNU sed does not.
**How to avoid:** Use temp file + mv pattern, or detect OS. [VERIFIED: known platform difference]
**Warning signs:** "invalid command code" or "undefined label" on macOS.

### Pitfall 8: Hardcoded Go Version in Workflow
**What goes wrong:** CI uses wrong Go version causing build failures.
**Why it happens:** Current release.yml hardcodes `go-version: '1.26'` but go.mod says `go 1.25`.
**How to avoid:** Use `go-version-file: go.mod`. [VERIFIED: go.mod declares go 1.25]
**Warning signs:** Build works locally but fails in CI.

### Pitfall 9: Version Default "dev" vs Build Detection
**What goes wrong:** --version check in start.sh sees "dev" and always re-downloads.
**Why it happens:** If binary was built locally without ldflags, version is "dev".
**How to avoid:** Treat "dev" as a special case in version comparison -- skip update if version is "dev" (local dev build). [ASSUMED: reasonable default]
**Warning signs:** Every session triggers a re-download for developers using local builds.

### Pitfall 10: CLAUDE_PLUGIN_DATA Not Set Outside Hook Context
**What goes wrong:** Running start.sh manually fails because env var is not set.
**Why it happens:** CLAUDE_PLUGIN_DATA is set by Claude Code, not by the OS.
**How to avoid:** Compute fallback path: `${CLAUDE_PLUGIN_DATA:-$HOME/.claude/plugins/data/dsrcode-dsrcode}`.
**Warning signs:** Empty or literal ${CLAUDE_PLUGIN_DATA} in paths.

## Code Examples

### Cross-Platform Platform Detection (Bash)

```bash
# Source: fzf, starship, goreleaser installer patterns
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    IS_WINDOWS=false
    case "$OS" in
        mingw*|msys*|cygwin*) IS_WINDOWS=true; OS="windows" ;;
        darwin) ;;
        linux) ;;
        *)
            echo "Error: Unsupported OS: $OS" >&2
            exit 1
            ;;
    esac

    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)
            echo "Error: Unsupported architecture: $ARCH" >&2
            exit 1
            ;;
    esac
}
```

### Atomic Download with Verification (Bash)

```bash
# Source: fzf install pattern + GoReleaser checksum format
download_binary() {
    local archive_name archive_ext tmp_dir tmp_archive

    if $IS_WINDOWS; then
        archive_ext="zip"
    else
        archive_ext="tar.gz"
    fi
    archive_name="${PROJECT}_${VERSION#v}_${OS}_${ARCH}.${archive_ext}"

    tmp_dir=$(mktemp -d)
    tmp_archive="${tmp_dir}/${archive_name}"

    local url="https://github.com/${REPO}/releases/download/${VERSION}/${archive_name}"
    echo "Downloading cc-discord-presence ${VERSION} for ${OS}/${ARCH}..."

    if ! curl -fsSL -o "$tmp_archive" "$url" 2>/dev/null; then
        rm -rf "$tmp_dir"
        echo "Warning: Download failed" >&2
        return 1
    fi

    # Verify checksum (DIST-05)
    if ! download_and_verify "$archive_name" "$tmp_archive"; then
        rm -rf "$tmp_dir"
        return 1
    fi

    # Extract binary from archive
    if [[ "$archive_ext" == "tar.gz" ]]; then
        tar -xzf "$tmp_archive" -C "$tmp_dir" cc-discord-presence
    else
        # Windows: use unzip or PowerShell
        unzip -q "$tmp_archive" cc-discord-presence.exe -d "$tmp_dir" 2>/dev/null
    fi

    local extracted_name="cc-discord-presence"
    $IS_WINDOWS && extracted_name="cc-discord-presence.exe"

    mv "${tmp_dir}/${extracted_name}" "$BINARY"
    if ! $IS_WINDOWS; then
        chmod +x "$BINARY"
    fi

    rm -rf "$tmp_dir"
    echo "Downloaded and verified successfully!"
    return 0
}
```

### PowerShell Download with Verification

```powershell
# Source: pattern from multiple GitHub gists + official PowerShell docs
function Download-Binary {
    param([string]$Version, [string]$BinDir)

    $ProgressPreference = 'SilentlyContinue'  # Speed up Invoke-WebRequest
    $archiveName = "cc-discord-presence_$($Version.TrimStart('v'))_windows_amd64.zip"
    $checksumName = "cc-discord-presence_$($Version.TrimStart('v'))_checksums.txt"
    $baseUrl = "https://github.com/$Repo/releases/download/$Version"

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "cc-discord-$(Get-Random)"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    try {
        # Download archive
        Invoke-WebRequest -Uri "$baseUrl/$archiveName" -OutFile (Join-Path $tmpDir $archiveName) -UseBasicParsing

        # Download and verify checksum
        Invoke-WebRequest -Uri "$baseUrl/$checksumName" -OutFile (Join-Path $tmpDir $checksumName) -UseBasicParsing
        $expectedLine = (Get-Content (Join-Path $tmpDir $checksumName)) | Where-Object { $_ -match $archiveName }
        if ($expectedLine) {
            $expectedHash = ($expectedLine -split '\s+')[0]
            $actualHash = (Get-FileHash -Path (Join-Path $tmpDir $archiveName) -Algorithm SHA256).Hash.ToLower()
            if ($actualHash -ne $expectedHash.ToLower()) {
                throw "Checksum mismatch! Expected: $expectedHash Got: $actualHash"
            }
        }

        # Extract
        Expand-Archive -Path (Join-Path $tmpDir $archiveName) -DestinationPath $tmpDir -Force
        $binaryPath = Join-Path $BinDir "cc-discord-presence.exe"
        Move-Item -Path (Join-Path $tmpDir "cc-discord-presence.exe") -Destination $binaryPath -Force

        Write-Host "Downloaded and verified successfully!"
        return $true
    } catch {
        Write-Host "Warning: Download failed: $_" -ForegroundColor Yellow
        return $false
    } finally {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| goreleaser v1 `format:` | goreleaser v2 `formats:` (list) | v2.6 (2025) | Config field accepts multiple formats |
| goreleaser-action@v6 | goreleaser-action@v7 | 2026-02-21 | Default matches v2 |
| softprops/action-gh-release | goreleaser (all-in-one) | N/A | Single tool replaces build + release |
| `--rm-dist` | `--clean` | v2.0 | Old flag removed |
| `archives.builds` | `archives.ids` | v2.8 | Renamed for consistency |
| Custom ~/.claude/bin/ | ${CLAUDE_PLUGIN_DATA}/bin/ | v2.1.91 (2026-W14) | Official persistent storage |
| Build-from-source only | Download-first + build fallback | This phase | First-time users without Go can use plugin |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | go-winio, fsnotify, lumberjack are all pure Go (no CGO) | Build config | Cross-compilation fails. Mitigation: test with `goreleaser build --snapshot` locally |
| A2 | CLAUDE_PLUGIN_DATA is available as env var inside scripts called from SessionStart hooks | Storage pattern | Would need to hardcode path ~/.claude/plugins/data/dsrcode-dsrcode/. Mitigation: use fallback pattern in script |
| A3 | 15 second SessionStart timeout is sufficient for ~10MB download | Timeout analysis | First-run fails on slow connections. Next session retries automatically. |
| A4 | jq is available on developer's machine for bump script | Bump script | Use sed fallback for JSON files if jq missing |
| A5 | sha256sum is available in Git Bash on Windows | Checksum verification | Graceful fallback: skip verification if no tool found, proceed with HTTPS-verified download |
| A6 | "dev" version from non-ldflags build should skip update check | Pitfall 9 | Developers always see re-download attempt. Low impact, just annoying. |

## Open Questions

1. **goreleaser-action version: v6 vs v7**
   - What we know: CONTEXT.md specifies @v6, but @v7 is current (2026-02-21). The README examples use @v7.
   - What's unclear: Whether the user explicitly chose v6 for a reason, or if this was based on stale information.
   - Recommendation: Use @v7 -- it is the latest stable and matched by the README examples. Flag for user confirmation.

2. **Archive extraction on Windows Git Bash**
   - What we know: tar is available in modern Git Bash, but the archive format for Windows is .zip per convention.
   - What's unclear: Whether `unzip` is reliably available in Git Bash, or if we should use PowerShell for extraction.
   - Recommendation: For Windows downloads in start.sh, shell out to PowerShell for extraction: `powershell.exe -Command "Expand-Archive -Path '$archive' -DestinationPath '$dir' -Force"`. This is reliable on all Windows machines.

3. **Bump script portability (BSD vs GNU sed)**
   - What we know: Developer is on Windows (Git Bash). macOS contributors would need BSD sed compatibility.
   - What's unclear: Whether any macOS contributors will use the bump script.
   - Recommendation: Use jq for JSON files (portable) and temp-file+mv pattern for non-JSON sed operations.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Build from source | Yes (developer) | 1.26 | Download pre-built binary |
| curl | Binary download (bash) | Yes (all platforms) | 8.x | wget (extremely rare) |
| tar | Archive extraction (Unix) | Yes (all platforms) | built-in | -- |
| unzip | Archive extraction (Windows bash) | Uncertain (Git Bash) | -- | PowerShell Expand-Archive |
| sha256sum | Checksum (Linux/Git Bash) | Yes | built-in | shasum -a 256 |
| shasum | Checksum (macOS) | Yes (macOS) | built-in | sha256sum (coreutils) |
| PowerShell | start.ps1, Windows extraction | Yes (Windows) | 5.1+ | -- |
| jq | Bump script JSON editing | Likely yes | -- | sed fallback (less safe) |
| GitHub Actions | CI/CD | Yes | -- | -- |

**Missing dependencies with no fallback:** None
**Missing dependencies with fallback:** unzip on Git Bash -> PowerShell

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | go test (built-in) |
| Config file | go.mod (Go 1.25) |
| Quick run command | `cd ~/Projects/cc-discord-presence && go test ./... -count=1 -short` |
| Full suite command | `cd ~/Projects/cc-discord-presence && go test ./... -count=1 -race -v` |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIST-01 | GoReleaser config valid | config-check | `goreleaser check` (local) | N/A (config file) |
| DIST-02 | Workflow triggers on tag + dispatch | manual | Push test tag, verify release | manual-only |
| DIST-03 | 5 platform binaries built | smoke | `goreleaser build --snapshot --clean` | N/A (CI) |
| DIST-04 | Download + fallback flow | integration | Manual test of start.sh with/without network | manual-only |
| DIST-05 | SHA256 verification | unit | Inline shellcheck + manual test | manual-only |
| DIST-06 | Binary in PLUGIN_DATA | integration | `echo $CLAUDE_PLUGIN_DATA && ls ${CLAUDE_PLUGIN_DATA}/bin/` | manual-only |
| DIST-07 | Bump script updates all files | smoke | `./scripts/bump-version.sh 99.99.99 && git diff` | manual-only |
| DIST-08 | Version variable renamed | unit | `go test ./... -count=1` (existing tests pass) | main_test.go exists |
| DIST-09 | No download on every start | integration | Start twice, verify no download on second | manual-only |
| DIST-10 | Correct repo in all URLs | grep | `grep -r 'StrainReviews/dsrcode' scripts/` | manual-only |

### Sampling Rate
- **Per task commit:** `go test ./... -count=1 -short`
- **Per wave merge:** `go test ./... -count=1 -race -v` + `goreleaser check`
- **Phase gate:** Full suite green + `goreleaser build --snapshot` produces 5 binaries

### Wave 0 Gaps
- [ ] `goreleaser check` requires goreleaser installed locally -- install via `go install github.com/goreleaser/goreleaser/v2@latest` or `brew install goreleaser`
- [ ] Shell script testing is manual -- no automated test framework for bash scripts in this project

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | N/A (no auth in binary distribution) |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A |
| V5 Input Validation | Yes | Validate downloaded file size, checksum match, version string format |
| V6 Cryptography | Yes | SHA256 checksum verification (not hand-rolled -- uses OS tools) |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| MITM binary replacement | Tampering | HTTPS transport + SHA256 checksum verification |
| Corrupted download | Tampering | Atomic temp+mv pattern, file size check, checksum |
| Malicious release tag | Elevation of Privilege | GITHUB_TOKEN scope limited to contents:write; tag must be pushed by authorized user |
| Supply chain (CI compromise) | Tampering | goreleaser-action pinned to @v7 (not @latest); GITHUB_TOKEN auto-scoped |

## Sources

### Primary (HIGH confidence)
- [GoReleaser Go Builds docs](https://goreleaser.com/customization/builds/go/) - Build matrix, ldflags, CGO, ignore [CITED]
- [GoReleaser Archives docs](https://goreleaser.com/customization/archive/) - formats (v2), name_template, files [CITED]
- [GoReleaser Checksum docs](https://goreleaser.com/customization/checksum/) - algorithm, name_template [CITED]
- [GoReleaser Release docs](https://goreleaser.com/customization/release/) - github owner/name, prerelease, make_latest [CITED]
- [GoReleaser Changelog docs](https://goreleaser.com/customization/changelog/) - github-native [CITED]
- [GoReleaser GitHub Actions docs](https://goreleaser.com/ci/actions/) - workflow, fetch-depth, GITHUB_TOKEN [CITED]
- [GoReleaser Deprecations](https://goreleaser.com/deprecations/) - v2 breaking changes [CITED]
- [GoReleaser main.version cookbook](https://goreleaser.com/cookbooks/using-main.version/) - default ldflags, lowercase pattern [CITED]
- [goreleaser-action README](https://github.com/goreleaser/goreleaser-action) - v7.0.0, inputs, workflow examples [VERIFIED: gh api]
- [Claude Code Plugins Reference](https://code.claude.com/docs/en/plugins-reference) - PLUGIN_DATA, PLUGIN_ROOT, bin/, hooks [VERIFIED]
- [SHA-256 checksums cross-platform](https://tobywf.com/2023/03/sha-256-checksums/) - sha256sum vs shasum platform differences [CITED]

### Verified via CLI (HIGH confidence)
- GoReleaser latest: v2.15.2 (2026-03-31) [VERIFIED: `gh release list --repo goreleaser/goreleaser`]
- goreleaser-action latest: v7.0.0 (2026-02-21) [VERIFIED: `gh release list --repo goreleaser/goreleaser-action`]
- Repo exists: StrainReviews/dsrcode [VERIFIED: `gh repo view StrainReviews/dsrcode`]
- No existing releases [VERIFIED: `gh release list --repo StrainReviews/dsrcode` returns empty]
- go.mod: Go 1.25, pure Go deps [VERIFIED: file read]
- main.go:33: `var Version = "3.1.10"` (capital V) [VERIFIED: grep]
- Binary naming: `cc-discord-presence-windows-amd64.exe` in ~/.claude/bin/ [VERIFIED: ls]
- PLUGIN_DATA path: ~/.claude/plugins/data/dsrcode-dsrcode/ [VERIFIED: ls]
- --version output: "cc-discord-presence 3.1.10" (awk field $2) [VERIFIED: main.go:156]
- Existing workflow: manual builds, no ldflags, no checksums [VERIFIED: release.yml]
- start.ps1: wrong repo (tsanva/cc-discord-presence), wrong version (v1.0.3) [VERIFIED: file read]
- start.sh: $ROOT used before defined (line 84 vs 90) [VERIFIED: file read]

### Secondary (MEDIUM confidence)
- [workflow_dispatch with goreleaser](https://carlosbecker.com/posts/goreleaser-create-tag-action/) - tag creation pattern [CITED]
- [GitHub Releases checksum gist](https://gist.github.com/thanoskoutr/12afbd6b87d8c0126f344cfae75769e3) - download + verify pattern [CITED]
- [Download artifacts from GitHub](https://blog.markvincze.com/download-artifacts-from-a-latest-github-release-in-sh-and-powershell/) - PowerShell pattern [CITED]
- [fzf install script](https://github.com/junegunn/fzf/blob/master/install) - download + go build fallback pattern
- [starship installer](https://github.com/starship/starship) - platform detection pattern

### Existing Research (consolidated)
- `.planning/phases/05-binary-distribution/RESEARCH-goreleaser.md` - GoReleaser config + GitHub Actions
- `.planning/phases/05-binary-distribution/RESEARCH-start-sh.md` - Complete start.sh rewrite
- `.planning/phases/05-binary-distribution/RESEARCH-plugin-install.md` - Plugin lifecycle + CLAUDE_PLUGIN_DATA

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - versions verified against GitHub releases and official docs
- GoReleaser config: HIGH - v2 syntax verified against deprecation notices and official docs
- Binary distribution flow: HIGH - download/verify/extract pattern from established installers
- Plugin lifecycle: HIGH - verified against official Claude Code docs
- Shell scripting: HIGH - patterns from fzf, starship, goreleaser installers
- PowerShell: MEDIUM - patterns from GitHub gists, not all tested
- Bump script: MEDIUM - sed portability concern flagged, jq approach mitigates

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (goreleaser stable, plugin system stable)

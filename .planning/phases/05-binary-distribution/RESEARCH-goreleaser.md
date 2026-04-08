# Phase 5: Binary Distribution via GoReleaser - Research

**Researched:** 2026-04-06
**Domain:** Go cross-compilation, release automation, GitHub Actions CI/CD
**Confidence:** HIGH

## Summary

GoReleaser v2 is the standard tool for automating Go binary cross-compilation and GitHub Release publishing. The `cc-discord-presence` project currently has a manual release workflow (`.github/workflows/release.yml`) that shells out raw `GOOS=/GOARCH=` builds without ldflags injection, checksum generation, or archive packaging. Replacing it with GoReleaser adds version injection via ldflags, SHA256 checksums, tar.gz/zip archives, and automatic GitHub Release notes -- all from a single declarative YAML file.

The project is pure Go (no CGO dependencies -- `go-winio`, `fsnotify`, `lumberjack` are all pure Go), which makes cross-compilation trivial. GoReleaser will build the cartesian product of the specified `goos` and `goarch` lists with `CGO_ENABLED=0`.

**Primary recommendation:** Add a `.goreleaser.yaml` (version 2 format) and replace the existing `release.yml` workflow with `goreleaser/goreleaser-action@v7`. The version variable at `main.go:33` (`var Version = "3.1.10"`) maps to goreleaser's default ldflag `-X main.Version={{.Version}}` -- but the variable is named `Version` (capital V), so a custom ldflag is needed since goreleaser's default targets lowercase `main.version`.

---

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| GoReleaser | v2.15.2 | Cross-compile, archive, checksum, publish | De facto standard for Go release automation [VERIFIED: `gh release list --repo goreleaser/goreleaser` showed v2.15.2 as Latest, published 2026-03-31] |
| goreleaser-action | v7.0.0 | GitHub Actions integration | Official action, default `version: '~> v2'` [VERIFIED: `gh repo view goreleaser/goreleaser-action` showed v7.0.0 published 2026-02-21] |
| actions/checkout | v4 | Git checkout with full history | Required with `fetch-depth: 0` for changelog [CITED: goreleaser-action README] |
| actions/setup-go | v5 | Go toolchain setup | Matches existing workflow; v6 also available [VERIFIED: existing release.yml uses v5] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| GoReleaser | Manual `go build` (current approach) | No checksums, no archives, no ldflags, no changelog -- current workflow is missing all of these |
| GoReleaser | `go-xbuild` | Newer, less mature, fewer features, smaller community |
| GoReleaser | `gox` + `ghr` | Two tools instead of one, no archive support, abandoned |

---

## Architecture Patterns

### GoReleaser v2 Config Format

GoReleaser v2 requires `version: 2` as the first meaningful line. Key v2-specific changes from v1: [CITED: goreleaser.com/deprecations/]

| v1 Field | v2 Field | Changed In |
|----------|----------|------------|
| `archives.format` | `archives.formats` (list) | v2.6 |
| `archives.format_overrides.format` | `archives.format_overrides.formats` (list) | v2.6 |
| `archives.builds` | `archives.ids` | v2.8 |
| `--rm-dist` flag | `--clean` flag | v2.0 |

### Cartesian Product Build Matrix

When you specify `goos: [darwin, linux, windows]` and `goarch: [amd64, arm64]`, GoReleaser builds ALL 6 combinations. Use `ignore` to exclude unwanted pairs (e.g., `windows/arm64`). [CITED: goreleaser.com/customization/builds/go/]

### ldflags Version Injection

GoReleaser sets three default ldflags automatically: [CITED: goreleaser.com/cookbooks/using-main.version/]

| Variable | Value | Template |
|----------|-------|----------|
| `main.version` | Git tag (v prefix stripped) | `{{.Version}}` |
| `main.commit` | Git commit SHA | `{{.Commit}}` |
| `main.date` | RFC3339 date | `{{.Date}}` |

**CRITICAL for cc-discord-presence:** The project uses `var Version` (capital V), not `var version` (lowercase). The default ldflag targets `main.version` (lowercase). You MUST use a custom ldflag:

```yaml
ldflags:
  - -s -w -X main.Version={{.Version}}
```

The `-s -w` flags strip debug info and DWARF tables, reducing binary size by ~30%.

### Archive Naming Convention

The standard naming template produces files like: [CITED: goreleaser.com/customization/archive/]

```
cc-discord-presence_1.0.0_darwin_arm64.tar.gz
cc-discord-presence_1.0.0_windows_amd64.zip
```

Template variables: `{{ .ProjectName }}`, `{{ .Version }}`, `{{ .Os }}`, `{{ .Arch }}`

---

## Ready-to-Use Configuration Files

### `.goreleaser.yaml`

```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
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
    ldflags:
      - -s -w -X main.Version={{.Version}}
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
    owner: DSR-Labs
    name: dsrcode
  prerelease: auto
  make_latest: true
```

**Build matrix produced (5 targets):**

| # | GOOS | GOARCH | Archive Format |
|---|------|--------|----------------|
| 1 | darwin | arm64 | tar.gz |
| 2 | darwin | amd64 | tar.gz |
| 3 | linux | amd64 | tar.gz |
| 4 | linux | arm64 | tar.gz |
| 5 | windows | amd64 | zip |

`windows/arm64` is excluded via `ignore` since it was not in the original target list.

**Key design decisions:**

- `CGO_ENABLED=0`: Pure Go, no C dependencies -- safe for all cross-compile targets [VERIFIED: go.mod shows only pure Go deps: go-winio, fsnotify, lumberjack]
- `mod_timestamp: "{{ .CommitTimestamp }}"`: Reproducible builds -- binary timestamps match git commit [CITED: goreleaser.com/blog/reproducible-builds/]
- `formats: [tar.gz]` with Windows override to `zip`: Linux/macOS users expect tar.gz, Windows users expect zip [CITED: goreleaser.com/customization/archive/]
- `changelog.use: github-native`: Delegates changelog to GitHub's auto-generated release notes [CITED: goreleaser.com/customization/changelog/]
- `prerelease: auto`: Tags like `v3.2.0-rc1` automatically marked as pre-release [CITED: goreleaser.com/customization/release/]
- `files` in archives includes LICENSE, README, etc. that exist in the repo root [VERIFIED: `ls` of cc-discord-presence showed these files]

### `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

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

**Key design decisions:**

- `fetch-depth: 0`: REQUIRED -- goreleaser needs full git history for changelog and version detection [CITED: goreleaser-action README, marked as "IMPORTANT"]
- `go-version-file: go.mod`: Reads Go version from `go.mod` (`go 1.25`) instead of hardcoding -- the existing workflow hardcodes `1.26` which is wrong [VERIFIED: go.mod says `go 1.25`]
- `version: "~> v2"`: Semver range -- gets latest v2.x.x (currently v2.15.2) without breaking on v3 [CITED: goreleaser-action README]
- `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`: Auto-provided by GitHub Actions, has `contents: write` scope from the `permissions` block [CITED: goreleaser.com/ci/actions/]
- Triggers on `v*` tags only -- matches the existing convention

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-compilation matrix | Shell loops with `GOOS=/GOARCH=` | GoReleaser `builds.goos`/`builds.goarch` | Current workflow has no ldflags, no checksums, no archives |
| Checksum generation | `sha256sum bin/*` in shell | GoReleaser `checksum` section | Automatic, includes all archives, standardized format |
| Archive packaging | `tar czf` / `zip` in shell | GoReleaser `archives` section | Per-OS format overrides, includes LICENSE/README automatically |
| Release notes | `generate_release_notes: true` on softprops/action-gh-release | GoReleaser `changelog.use: github-native` | Same result, but integrated into one tool |
| Version injection | Manual `-ldflags` in each `go build` | GoReleaser `builds.ldflags` | Template variables (`{{.Version}}`, `{{.Commit}}`, `{{.Date}}`) |

---

## Common Pitfalls

### Pitfall 1: Missing `fetch-depth: 0` in Checkout

**What goes wrong:** GoReleaser generates empty or incorrect changelog, version detection fails.
**Why it happens:** Default `actions/checkout` does a shallow clone (depth 1). GoReleaser needs full history to walk tags.
**How to avoid:** Always set `fetch-depth: 0` in the checkout step.
**Warning signs:** "changelog: could not get tag" errors in CI logs.

### Pitfall 2: Capital V `Version` vs lowercase `version`

**What goes wrong:** Version shows as "dev" or empty string in the built binary.
**Why it happens:** GoReleaser's default ldflag targets `main.version` (lowercase). The project declares `var Version` (uppercase). Go's `ldflags -X` is case-sensitive.
**How to avoid:** Use explicit `ldflags: ["-s -w -X main.Version={{.Version}}"]` in `.goreleaser.yaml`.
**Warning signs:** Binary reports "dev" or "3.1.10" (hardcoded default) instead of the git tag version.

### Pitfall 3: Using v1 `format:` instead of v2 `formats:`

**What goes wrong:** GoReleaser v2.6+ warns about deprecated field, may error in future versions.
**Why it happens:** Most blog posts and StackOverflow answers still show v1 syntax.
**How to avoid:** Always use `formats: ["tar.gz"]` (list) and `format_overrides.formats: ["zip"]` (list).
**Warning signs:** Deprecation warning in goreleaser output.

### Pitfall 4: Hardcoded Go Version in Workflow

**What goes wrong:** CI uses a different Go version than the project requires, causing build failures or subtle incompatibilities.
**Why it happens:** The existing workflow hardcodes `go-version: '1.26'` but `go.mod` declares `go 1.25`.
**How to avoid:** Use `go-version-file: go.mod` to read the version from the source of truth.
**Warning signs:** Build works locally but fails in CI, or vice versa.

### Pitfall 5: CGO_ENABLED Not Explicitly Disabled

**What goes wrong:** Cross-compilation fails with linker errors for `darwin` or `linux/arm64` targets.
**Why it happens:** On some CI runners, `CGO_ENABLED=1` is the default. Cross-compiling with CGO requires a cross-compiler toolchain.
**How to avoid:** Always set `env: [CGO_ENABLED=0]` in the build config for pure Go projects.
**Warning signs:** `gcc: error: unrecognized command-line option` or `cannot find -lgcc` in build output.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `goreleaser v1` with `format:` | `goreleaser v2` with `formats:` (list) | v2.6 (2025) | Config field renamed, accepts multiple formats |
| `goreleaser-action@v6` | `goreleaser-action@v7` | 2026-02-21 | Matches v2 defaults |
| `softprops/action-gh-release` | `goreleaser` (all-in-one) | N/A | Single tool replaces build + release |
| `--rm-dist` flag | `--clean` flag | v2.0 | Old flag removed |
| `archives.builds` | `archives.ids` | v2.8 | Renamed for consistency |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `go-winio`, `fsnotify`, `lumberjack` are all pure Go (no CGO) | Builds config | If any require CGO, cross-compilation with `CGO_ENABLED=0` will fail at link time. Mitigation: test with `goreleaser build --snapshot` locally. |
| A2 | The GitHub repo for releases is `DSR-Labs/dsrcode` | Release config | If the repo name is different, the `release.github` block needs updating. This was stated in the task context but not independently verified. |
| A3 | CHANGELOG.md, PRIVACY.md, TERMS.md exist in repo root and should be included in archives | Archives config | If any file is missing, goreleaser will warn but not fail (glob pattern). Can be adjusted. |

---

## Open Questions

1. **Repository name confirmation**
   - What we know: Task context says "Repo: DSR-Labs/dsrcode on GitHub"
   - What's unclear: Whether the binary repo is `dsrcode` or a separate repo like `cc-discord-presence`
   - Recommendation: Verify with `gh repo view` before committing the config. If wrong, update `release.github.owner` and `release.github.name`.

2. **Go 1.25 availability in actions/setup-go**
   - What we know: `go.mod` says `go 1.25`. The existing workflow uses `go-version: '1.26'`.
   - What's unclear: Whether Go 1.25 is a released stable version or a development version. The `go-version-file: go.mod` approach handles this automatically.
   - Recommendation: Use `go-version-file: go.mod` and let setup-go resolve it.

3. **Whether to keep the old workflow or replace it**
   - What we know: The existing `release.yml` does manual `go build` to 5 platforms with `softprops/action-gh-release`.
   - What's unclear: Whether there are downstream consumers expecting the old artifact naming pattern (`cc-discord-presence-darwin-arm64` without version).
   - Recommendation: Replace entirely. The new naming (`cc-discord-presence_X.Y.Z_darwin_arm64.tar.gz`) is more standard and includes versioning.

---

## Sources

### Primary (HIGH confidence)
- [GoReleaser Go Builds docs](https://goreleaser.com/customization/builds/go/) - Build config, goos/goarch matrix, ldflags, CGO, ignore field
- [GoReleaser Archives docs](https://goreleaser.com/customization/archive/) - formats, format_overrides, name_template, files
- [GoReleaser Checksum docs](https://goreleaser.com/customization/checksum/) - algorithm, name_template
- [GoReleaser Changelog docs](https://goreleaser.com/customization/changelog/) - use: github-native
- [GoReleaser Release docs](https://goreleaser.com/customization/release/) - github owner/name, prerelease: auto, make_latest
- [GoReleaser GitHub Actions docs](https://goreleaser.com/ci/actions/) - workflow setup, fetch-depth, GITHUB_TOKEN
- [GoReleaser Deprecations](https://goreleaser.com/deprecations/) - v2 breaking changes, format->formats, builds->ids
- [GoReleaser v2 announcement](https://goreleaser.com/blog/goreleaser-v2/) - version: 2 header requirement, migration from v1
- [GoReleaser main.version cookbook](https://goreleaser.com/cookbooks/using-main.version/) - default ldflags, Go variable declaration pattern
- [goreleaser-action README](https://github.com/goreleaser/goreleaser-action) - v7.0.0, inputs table, workflow examples (fetched via `gh api`)

### Verified via CLI (HIGH confidence)
- GoReleaser latest release: v2.15.2 (2026-03-31) -- `gh release list --repo goreleaser/goreleaser`
- goreleaser-action latest: v7.0.0 (2026-02-21) -- `gh repo view goreleaser/goreleaser-action`
- cc-discord-presence `go.mod`: Go 1.25, pure Go deps only -- direct file read
- cc-discord-presence `main.go:33`: `var Version = "3.1.10"` (capital V) -- grep confirmed
- Existing `release.yml`: manual builds, no ldflags, no checksums -- direct file read

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - versions verified against GitHub releases and official docs
- Configuration format: HIGH - v2 syntax verified against deprecation notices and official docs
- ldflags pattern: HIGH - capital V issue confirmed by reading actual source code
- Pitfalls: HIGH - derived from official docs and direct observation of existing workflow gaps

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (goreleaser is stable, v2 format unlikely to change)

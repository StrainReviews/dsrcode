# Phase 5: Binary Distribution Pipeline + Full dsrcode Rename - Context

**Gathered:** 2026-04-09 (re-discussed, replaces 2026-04-06 context)
**Status:** Ready for planning

<domain>
## Phase Boundary

Binary distribution via GoReleaser + GitHub Releases, combined with a full project rename from cc-discord-presence to dsrcode. Produces v4.0.0 as a major breaking release with automatic migration for existing users. start.sh/start.ps1 download pre-built binaries with SHA256 verification. Go module path changes from tsanva/cc-discord-presence to StrainReviews/dsrcode. All runtime files (config, pid, log, sessions, analytics) renamed from discord-presence-* to dsrcode-*. CI gets test workflow + golangci-lint.

Repo: StrainReviews/dsrcode (GitHub) — permanent, no transfer planned.
Binary: dsrcode (Go, pure Go dependencies, CGO_ENABLED=0)
Version: v4.0.0 (major bump for breaking rename)

</domain>

<decisions>
## Implementation Decisions

### Repository & Identity (DIST-01 to DIST-03)
- **DIST-01:** Repo stays at StrainReviews/dsrcode permanently. No transfer to DSR-Labs. Phase 7 (Repo Transfer) removed from roadmap.
- **DIST-02:** Binary renamed from cc-discord-presence to dsrcode[.exe]. GoReleaser `binary: dsrcode`, `project_name: dsrcode`.
- **DIST-03:** Go module path: github.com/StrainReviews/dsrcode (was github.com/tsanva/cc-discord-presence). All imports updated via `go mod edit -module` + find-replace.

### Version & Release Strategy (DIST-04 to DIST-08)
- **DIST-04:** Version v4.0.0 — major bump for breaking changes (binary name, config paths, module path). Semver-korrekt.
- **DIST-05:** Single release combining GoReleaser distribution + full dsrcode rename. No phased approach.
- **DIST-06:** var Version (uppercase) renamed to var version = "dev" (lowercase) for GoReleaser default ldflags. Also add var commit = "none" and var date = "unknown" for enhanced --version output.
- **DIST-07:** GoReleaser default ldflags inject version, commit, date. --version output: "dsrcode 4.0.0 (abc1234, 2026-04-09)".
- **DIST-08:** prerelease: auto in .goreleaser.yaml. Tags like v4.0.0-rc1 auto-marked as pre-release. make_latest: true for stable releases.

### GoReleaser Configuration (DIST-09 to DIST-13)
- **DIST-09:** goreleaser-action@v7 (latest, Node 24 + ESM). version: "~> v2" for GoReleaser v2.15+.
- **DIST-10:** go-version-file: go.mod in CI (reproducible builds, matches dev environment).
- **DIST-11:** Archive format: tar.gz for Unix, zip for Windows (format_overrides). Includes LICENSE, README.md, CHANGELOG.md, PRIVACY.md, TERMS.md.
- **DIST-12:** SHA256 checksums (default algorithm). No signing (GPG/Cosign). Sufficient for plugin scope.
- **DIST-13:** Changelog: use: github with groups (feat/fix/docs). Filters out test/ci/chore commits. Shows author usernames.

### CI/CD Pipeline (DIST-14 to DIST-17)
- **DIST-14:** Release workflow triggers: tag push (v*) AND workflow_dispatch (manual release from GitHub UI).
- **DIST-15:** Separate test workflow (test.yml) for PRs: go vet + go test + golangci-lint.
- **DIST-16:** .golangci.yml configuration for automated linting in CI.
- **DIST-17:** dist/ added to .gitignore (GoReleaser build artifacts).

### Binary Storage & Plugin Integration (DIST-18 to DIST-22)
- **DIST-18:** Binary stored in ${CLAUDE_PLUGIN_DATA}/bin/dsrcode[.exe]. CLAUDE_PLUGIN_DATA is official, persistent, auto-cleanup on uninstall.
- **DIST-19:** CLAUDE_PLUGIN_DATA fallback: $HOME/.claude/plugins/data/dsrcode (for --plugin-dir dev mode).
- **DIST-20:** hooks.json does NOT need modification. CLAUDE_PLUGIN_DATA is automatically exported as env var to hook processes per official Claude Code docs.
- **DIST-21:** Download only when binary missing or version mismatch. Version check via --version is <100ms. SessionStart 15s timeout sufficient.
- **DIST-22:** "dev" and "unknown" version values skip update check (local dev builds).

### Start Scripts (DIST-23 to DIST-28)
- **DIST-23:** start.sh complete rewrite: download-first + build-fallback + SHA256 verification + CLAUDE_PLUGIN_DATA storage. Preserve ALL existing logic (session tracking, health check, hooks autopatch, first-run message, statusline check).
- **DIST-24:** start.ps1 complete rewrite: same logic in PowerShell. Uses Get-FileHash (SHA256), Expand-Archive, Invoke-WebRequest. All built-in, no external deps.
- **DIST-25:** stop.sh and stop.ps1 complete overhaul: new dsrcode.pid path + fallback on old discord-presence.pid. SIGTERM → wait → SIGKILL pattern.
- **DIST-26:** Lock-file in start.sh/ps1 during download/update to prevent concurrent update race conditions (flock on Unix, lock file on Windows).
- **DIST-27:** Error messages in English (international plugin). Proxy hint on download failure: "If behind a proxy, set HTTP_PROXY/HTTPS_PROXY."
- **DIST-28:** statusline-wrapper.sh auto-updated by start.sh (checks if wrapper is current, copies new version).

### Full dsrcode Rename (DIST-29 to DIST-36)
- **DIST-29:** ALL runtime files renamed: dsrcode-config.json, dsrcode-data.json, dsrcode.log, dsrcode.pid, dsrcode.refcount, dsrcode-sessions/, dsrcode-analytics/.
- **DIST-30:** Config migration: Go code checks new name first, falls back to old discord-presence-* name. On first find of old name: copy to new, log warning. Old files remain as backup.
- **DIST-31:** Backward compatibility: 6 months (v4.x supports old file names, v5.0 removes fallback).
- **DIST-32:** All 4 skills updated (doctor, update, setup, log): new binary paths, new config paths, correct repo URL (StrainReviews/dsrcode).
- **DIST-33:** dsrcode:update skill: start.sh has priority for version management. Skill is for manual on-demand updates. Lock-file prevents race condition between both.
- **DIST-34:** All Go source: import paths updated from tsanva/cc-discord-presence to StrainReviews/dsrcode. go.sum regenerated via go mod tidy.
- **DIST-35:** --version output: "dsrcode X.Y.Z (commit, date)" instead of "cc-discord-presence X.Y.Z".
- **DIST-36:** slog messages: "starting dsrcode" instead of "starting cc-discord-presence".

### Migration & Compatibility (DIST-37 to DIST-40)
- **DIST-37:** Auto-migration in start.sh: detect old binary at ~/.claude/bin/cc-discord-presence-*, kill running daemon, move to CLAUDE_PLUGIN_DATA/bin/dsrcode, delete old. Log migration for transparency.
- **DIST-38:** Windows arm64: ignored in GoReleaser config. x86 emulation handles amd64 binary. 5 platforms: macOS arm64+amd64, Linux arm64+amd64, Windows amd64.
- **DIST-39:** Bootstrap first release: workflow_dispatch to manually trigger v4.0.0 after merge. Tag + push workflow.
- **DIST-40:** HTTP port 19460: stays unchanged. Changing would break existing hook configs.

### Version Management (DIST-41 to DIST-43)
- **DIST-41:** bump-version.sh: updates 5 files (main.go, plugin.json, marketplace.json, start.sh, start.ps1). Prints git commit/tag instructions. Does NOT auto-commit.
- **DIST-42:** Bash-only bump script. No PowerShell equivalent needed (Git Bash standard on Windows).
- **DIST-43:** build.sh: REMOVED. GoReleaser replaces it. `goreleaser build --snapshot --clean` for local builds.

### Documentation (DIST-44 to DIST-48)
- **DIST-44:** CLAUDE.md: Releasing section updated for GoReleaser workflow (bump-version.sh → tag → push → CI).
- **DIST-45:** CONTRIBUTING.md: updated with dsrcode references (clone URL, build commands, binary name).
- **DIST-46:** README.md: badges updated for new module path (Go Reference, Go Report Card). Installation instructions updated.
- **DIST-47:** MIGRATION.md: created for v4.0.0. Documents: old → new file names, binary name change, what auto-migrates, what needs manual action.
- **DIST-48:** GitHub repo description and topics: updated post-release.

### Project Infrastructure (DIST-49 to DIST-50)
- **DIST-49:** .editorconfig: created for editor consistency (indent, line-ending, charset).
- **DIST-50:** Import-update + new migration tests: config migration (old → new names), download fallback, version check skip for "dev".

### Deferred to Post-Phase 5
- **Projektordner-Rename:** C:\Users\ktown\Projects\cc-discord-presence → C:\Users\ktown\Projects\dsrcode. Done AFTER Phase 5 implementation + v4.0.0 release. Claude Code memory manually migrated.
- **Phase 7:** REMOVED from roadmap (repo stays at StrainReviews/dsrcode permanently).
- **Discord App:** Already correctly configured as "DSR Code" with ClientID 1489600745295708160. No changes needed.

### Claude's Discretion
- GoReleaser YAML config details (archive naming template, exact format)
- Exact golangci-lint configuration (which linters enabled)
- .editorconfig exact settings
- start.sh/ps1 exact function order and error handling patterns
- Migration test implementation details
- Exact MIGRATION.md content
- Release notes header/footer customization

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Source Files
- `main.go` — Version variable (line 34), --version output (line 152), imports (lines 23-30)
- `config/config.go` — DefaultConfigPath, DefaultLogPath (lines 136-152)
- `scripts/start.sh` — Current startup script (~300 lines, complete rewrite)
- `scripts/start.ps1` — Current PS script (completely outdated, complete rewrite)
- `scripts/stop.sh` — PID file reference
- `scripts/stop.ps1` — PID/refcount/process name references
- `scripts/statusline-wrapper.sh` — data file reference
- `scripts/setup-statusline.sh` — binary path references
- `hooks/hooks.json` — SessionStart hook config (NO changes needed for CLAUDE_PLUGIN_DATA)
- `.claude-plugin/plugin.json` — Plugin manifest, version field
- `.claude-plugin/marketplace.json` — Marketplace manifest, version field
- `.github/workflows/release.yml` — Current release workflow (replace with GoReleaser)
- `go.mod` — Module path (needs updating)

### Skills
- `_skills/doctor/SKILL.md` — Binary path, config path references
- `_skills/update/SKILL.md` — Binary path, repo URL (currently wrong: DSR-Labs)
- `_skills/setup/SKILL.md` — Binary path, config path references
- `_skills/log/SKILL.md` — Log file path references (5 occurrences)

### Documentation
- `CLAUDE.md` — Releasing section, build commands, project structure
- `CONTRIBUTING.md` — Clone URL, build commands (4 occurrences)
- `README.md` — Installation, badges, module references

### Research (from original discuss)
- `.planning/phases/05-binary-distribution/05-RESEARCH.md` — Original research (partially outdated)
- `.planning/phases/05-binary-distribution/RESEARCH-goreleaser.md` — GoReleaser config (needs update: @v6→@v7, project_name)
- `.planning/phases/05-binary-distribution/RESEARCH-start-sh.md` — start.sh research (needs update: dsrcode names)
- `.planning/phases/05-binary-distribution/RESEARCH-plugin-install.md` — CLAUDE_PLUGIN_DATA pattern (still valid)

### External Documentation (crawled 2026-04-09)
- Claude Code Plugins Reference: CLAUDE_PLUGIN_DATA, CLAUDE_PLUGIN_ROOT, plugin lifecycle
- Claude Code Hooks Guide: 26 hook types, SessionStart, CLAUDE_ENV_FILE, timeout 10min default
- GoReleaser v2.15 Docs: builds, archives, checksums, changelog, release config
- GoReleaser Cookbook: main.version ldflags pattern (version, commit, date)

</canonical_refs>

<code_context>
## Existing Code Insights

### Files Requiring Rename (discord-presence → dsrcode)
- **Go source:** main.go, config/config.go, all *_test.go files (import paths + string literals)
- **Scripts:** start.sh, start.ps1, stop.sh, stop.ps1, statusline-wrapper.sh, setup-statusline.sh
- **Skills:** 4 SKILL.md files (doctor, update, setup, log)
- **Docs:** CLAUDE.md, CONTRIBUTING.md, README.md
- **No rename needed:** demo, status, preset skills (no old references), hooks.json (no binary name), presets (embedded, no name references)

### Key Technical Facts
- go-winio uses pure Go (golang.org/x/sys/windows), NOT CGO. CGO_ENABLED=0 is safe.
- Discord App ClientID 1489600745295708160 is already "DSR Code". No change needed.
- HTTP port 19460 stays unchanged (hook configs depend on it).
- CLAUDE_PLUGIN_DATA auto-exported to hook processes — no hooks.json modification needed.

### Integration Points
- SessionStart hook → start.sh → downloads/starts dsrcode binary
- Config hot-reload via fsnotify → watches dsrcode-config.json (with discord-presence-config.json fallback)
- Statusline wrapper → writes dsrcode-data.json (with discord-presence-data.json fallback)

</code_context>

<specifics>
## Specific Ideas

- Go module rename script: `go mod edit -module github.com/StrainReviews/dsrcode && find . -name '*.go' -exec sed -i 's,tsanva/cc-discord-presence,StrainReviews/dsrcode,g' {} \; && go mod tidy`
- GoReleaser default ldflags with lowercase version/commit/date eliminates custom ldflag config
- Atomic download via temp file + mv (pattern from fzf, starship, goreleaser installers)
- Lock-file (flock) prevents concurrent binary updates when multiple sessions start simultaneously
- Config migration: check new name first, fall back to old name — copy on first migration, log warning

</specifics>

<deferred>
## Deferred Ideas

- Homebrew Tap for `brew install dsrcode` (needs separate homebrew-dsrcode repo, creates parallel install path)
- Scoop Bucket for `scoop install dsrcode` (similar, Windows only)
- Cosign keyless signing for releases (SHA256 checksums sufficient for now)
- Projektordner-Rename cc-discord-presence → dsrcode (after v4.0.0 release, manual Claude Code memory migration)
- GitHub Repo Description/Topics update (after v4.0.0 release)
- SessionEnd hook for daemon cleanup (Phase 6 scope)
- PreCompact/PostCompact hooks (Phase 6 scope)

</deferred>

---

*Phase: 05-binary-distribution*
*Context gathered: 2026-04-09 via discuss-phase (re-discussed with 50 MCP-backed decisions)*

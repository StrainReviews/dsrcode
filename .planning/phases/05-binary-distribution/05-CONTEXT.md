# Phase 5: Binary Distribution Pipeline - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Erstnutzer ohne Go-Compiler bekommen ein vorkompiliertes Binary via GitHub Releases. CI/CD baut per GoReleaser bei jedem Tag automatisch Binaries fuer 5 Plattformen. start.sh und start.ps1 laden Binary herunter mit Build-from-Source-Fallback. Version-Management ueber Bump-Script.

Repo: StrainReviews/dsrcode (GitHub)
Binary: cc-discord-presence (Go, pure Go dependencies, CGO_ENABLED=0)

</domain>

<decisions>
## Implementation Decisions

### Release-Strategie (DIST-01 bis DIST-03)
- **DIST-01:** GoReleaser v2.15+ mit goreleaser-action@v7 in GitHub Actions
- **DIST-02:** Trigger: Tag-Push (v*) UND workflow_dispatch (GitHub UI Release-Button)
- **DIST-03:** 5 Plattformen: macOS arm64/amd64, Linux amd64/arm64, Windows amd64. CGO_ENABLED=0 Cross-Compilation.

### Download vs Build Fallback (DIST-04 bis DIST-05)
- **DIST-04:** Download-first Strategie: 1) curl von GitHub Releases (atomic via temp file + mv), 2) go build Fallback wenn Go installiert, 3) Fehlermeldung mit Installationsanleitung. set -e erst NACH Binary-Acquisition.
- **DIST-05:** SHA256-Checksum-Verifikation nach Download. GoReleaser generiert checksums.txt automatisch. sha256sum/shasum vorinstalliert auf allen Plattformen.

### Binary-Speicherort (DIST-06)
- **DIST-06:** Binary in ${CLAUDE_PLUGIN_DATA}/bin/ speichern (offizielles Claude Code Plugin-Verzeichnis, ueberlebt Plugin-Updates, wird bei Uninstall aufgeraeumt). NICHT in ~/.claude/bin/ oder ${CLAUDE_PLUGIN_ROOT}.

### Version-Management (DIST-07 bis DIST-08)
- **DIST-07:** Bump-Script (scripts/bump-version.sh) nimmt Version als Argument, updated main.go + plugin.json + marketplace.json + start.sh + start.ps1 per sed. Dann git commit + git tag.
- **DIST-08:** Go Variable zu `var version` (lowercase) aendern fuer GoReleaser Default-ldflags (-X main.version={{.Version}}). Kein custom ldflag noetig.

### SessionStart Timeout (DIST-09)
- **DIST-09:** Download nur bei fehlendem Binary oder Version-Mismatch. Kein Download bei jedem Start. Version-Check per --version ist <100ms. Dadurch kein Timeout-Problem (15s SessionStart-Limit).

### Repo-Name (DIST-10)
- **DIST-10:** StrainReviews/dsrcode behalten. Existiert bereits, keine Migration noetig. GitHub Releases URL: https://github.com/StrainReviews/dsrcode/releases/download/{tag}/{binary}

### start.ps1 Update
- start.ps1 komplett rewriten mit gleicher Logik wie start.sh: Download-first + Build-Fallback + Version-Check + ${CLAUDE_PLUGIN_DATA}/bin/ Speicherort. Aktuell veraltet (tsanva/cc-discord-presence, v1.0.3).

### Migration bestehender Nutzer
- Beim ersten Start mit neuem start.sh: Binary von ~/.claude/bin/ nach ${CLAUDE_PLUGIN_DATA}/bin/ verschieben (move + cleanup). Altes Binary loeschen.

### Sprache & Auto-Update
- Fehlermeldungen in start.sh/start.ps1 auf **Englisch** (internationales Plugin, konsistent mit README und Go-Oekosystem)
- Bei Version-Mismatch: **Auto-Download** der neuen Version. Altes Binary ersetzen, kein User-Eingriff noetig.

### Claude's Discretion
- Goreleaser YAML Config Details (archive format, naming template)
- Exact checksums verification implementation (sha256sum vs shasum detection)
- start.sh/start.ps1 error message wording
- Bump-Script implementation details (sed patterns, commit message format)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Research Documents
- `.planning/phases/05-binary-distribution/RESEARCH-goreleaser.md` -- GoReleaser config + GitHub Actions workflow
- `.planning/phases/05-binary-distribution/RESEARCH-start-sh.md` -- Complete rewritten start.sh with download + fallback
- `.planning/phases/05-binary-distribution/RESEARCH-plugin-install.md` -- Plugin install lifecycle + CLAUDE_PLUGIN_DATA pattern

### Source Files (cc-discord-presence repo)
- `scripts/start.sh` -- Current bash startup script (needs rewrite)
- `scripts/start.ps1` -- Current PowerShell startup script (needs rewrite)
- `main.go:33` -- `var Version` (needs rename to lowercase)
- `.claude-plugin/plugin.json` -- Plugin manifest with version field
- `.claude-plugin/marketplace.json` -- Marketplace manifest with version
- `hooks/hooks.json` -- Hook config (SessionStart timeout: 15)
- `.github/workflows/release.yml` -- Current release workflow (replace with goreleaser)

### External Documentation
- GoReleaser v2 docs: https://goreleaser.com/customization/builds/go/
- Claude Code plugins-reference: https://code.claude.com/docs/en/plugins-reference (CLAUDE_PLUGIN_DATA pattern)

</canonical_refs>

<specifics>
## Specific Ideas

- GoReleaser default ldflags mit lowercase `version` eliminiert custom ldflag Config
- ${CLAUDE_PLUGIN_DATA} Pattern aus offizieller Claude Code Doku: diff manifest -> reinstall if changed
- Atomic download via temp file + mv (Pattern von fzf, starship, goreleaser installers)
- Bug in aktuellem start.sh: $ROOT wird referenziert bevor es definiert ist (Zeile 84 vs 90)

</specifics>

<deferred>
## Deferred Ideas

- DSR-Labs Organisation auf GitHub erstellen und Repo transferieren (nicht in Phase 5 Scope)
- Automatischer Changelog aus Conventional Commits (GoReleaser kann das, aber Konfiguration optional)
- Plugin bin/ Directory Feature nutzen (v2.1.91) fuer bare command Zugriff

</deferred>

---

*Phase: 05-binary-distribution*
*Context gathered: 2026-04-06 via discuss-phase*

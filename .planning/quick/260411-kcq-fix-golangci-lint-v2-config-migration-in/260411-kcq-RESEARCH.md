# Quick Task: Migrate `.golangci.yml` to golangci-lint v2 — Research

**Researched:** 2026-04-11
**Domain:** golangci-lint v2 config schema
**Confidence:** HIGH (primary source: official migration guide + v2 config file reference)

## Summary

CI uses `golangci-lint-action@v7` with `version: latest`, which installs golangci-lint **v2.11.4**. The checked-in `.golangci.yml` is still in v1 format and is missing the now-mandatory `version: "2"` header, which is exactly what the CI error reports (`unsupported version of the configuration: ""`).

The current v1 file asks for `[errcheck, govet, staticcheck, unused, ineffassign, gosimple, typecheck]`. In v2: `gosimple` has been merged into `staticcheck`, `typecheck` is no longer a user-selectable linter (it's always implicit), and the remaining 5 linters are **exactly** the v2 "standard" default set. So the new file can be minimal.

**Primary recommendation:** Replace `.golangci.yml` with the 8-line v2 equivalent below. No linter selection is needed at all — the v2 defaults match the intent of the old config.

## User Constraints

- **Scope = make CI green, not introduce new findings.** Do not silently enable extra linters beyond what the v1 config enabled.
- **Single-file change.** Touch only `.golangci.yml`. CI workflow already pins `golangci-lint-action@v7 / version: latest`, which is correct for v2.
- **Don't hand-roll:** an official `golangci-lint migrate` command exists but is not required here — the config is small enough to rewrite directly.

## Recommended v2 `.golangci.yml` (drop-in replacement)

```yaml
# golangci-lint v2 configuration for CI
# Run: golangci-lint run ./...
version: "2"

linters:
  default: none
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - ineffassign
  exclusions:
    presets:
      - comments
      - common-false-positives
      - legacy
      - std-error-handling
```

### Why this shape

- `version: "2"` — now **mandatory**; absence is the exact error CI is throwing. [VERIFIED: golangci-lint.run/docs/product/migration-guide]
- `linters.default: none` + explicit `enable:` list — this preserves v1 intent ("only these linters, nothing more"). Equivalent to v1 `disable-all: true` + `enable:`. [VERIFIED: golangci-lint.run migration guide table]
- `gosimple` removed — merged into `staticcheck` in v2. Listing it produces an "unknown linter" error. [VERIFIED: migration guide § Major Linter Changes]
- `typecheck` removed — "Not a linter — cannot be enabled/disabled." Always runs implicitly. [VERIFIED: migration guide FAQ note]
- `run.timeout: 5m` removed — in v2 there is no timeout by default (`0`) and the migration guide explicitly says the existing value is ignored. Leaving it in is harmless but noisy; the project has no reason to cap CI. [VERIFIED: migration guide § run.timeout]
- `issues.exclude-use-default: true` → `linters.exclusions.presets: [...]`. The four presets `comments`, `common-false-positives`, `legacy`, `std-error-handling` are the v2 equivalent of the old "default excludes" set. [VERIFIED: migration guide § exclude-use-default Migration]

### Alternative: even shorter (uses v2 "standard" defaults)

Because the 5 enabled linters the project wants are **exactly** the v2 standard default set (errcheck, govet, staticcheck, unused, ineffassign) [VERIFIED: golangci-lint.run/docs/welcome/quick-start], this works too:

```yaml
version: "2"

linters:
  exclusions:
    presets:
      - comments
      - common-false-positives
      - legacy
      - std-error-handling
```

**Recommendation:** prefer the explicit form (first block above). It's self-documenting and survives any future change to what "standard" means upstream.

## Per-Linter Migration Notes

| v1 name | v2 status | Action |
|---|---|---|
| `errcheck` | Unchanged, in standard set | Keep |
| `govet` | Unchanged, in standard set | Keep |
| `staticcheck` | Absorbed `gosimple` + `stylecheck` | Keep |
| `unused` | Unchanged, in standard set | Keep |
| `ineffassign` | Unchanged, in standard set | Keep |
| `gosimple` | **Merged into `staticcheck`** | **Remove** |
| `typecheck` | **No longer user-selectable** | **Remove** |

## Default-Linter Behavior Change Risk

Because we explicitly set `linters.default: none` and list only the 5 linters, **no new linters will fire**. The v2 `standard` default set is identical to the project's intended set, so even if someone dropped the `default: none` line, there would be no surprise findings from this migration alone.

One nuance: `gosimple`'s old checks now run under `staticcheck`, so *technically* CI was already running those checks implicitly via `gosimple`. Net-new findings after migration: **none expected**. [ASSUMED — validated only by reading release notes, not by actually running both versions against the repo; true test is the CI run post-merge.]

## Pitfalls

1. **`version` must be a string, not a number.** `version: 2` (unquoted) parses as int in YAML; v2 requires the string `"2"`. [VERIFIED: migration guide]
2. **`exclude-use-default: true` silently ignored.** If left in the file under `issues:`, v2 won't error but also won't apply any exclusions — moving it to `linters.exclusions.presets` is the only correct path. [VERIFIED: migration guide table]
3. **`linters-settings:` is gone.** It's now `linters.settings:` (nested). The current file has none, so not an issue here — flagging in case future edits re-introduce per-linter config.
4. **`run.timeout` is harmless but stale.** Keeping it won't break CI; removing it matches v2 intent.
5. **`golangci-lint migrate` tool exists** for automated migration but strips comments. For an 18-line file, manual rewrite is faster and safer. [CITED: migration guide § Migration Command]
6. **CI action version matters.** `golangci/golangci-lint-action@v7` is the v2-compatible action line. The current workflow already uses v7 + `version: latest`, so no workflow change is needed. [VERIFIED: `.github/workflows/test.yml` line 31–33]

## Sources

### Primary (HIGH confidence)
- https://golangci-lint.run/docs/product/migration-guide — authoritative v1→v2 migration table (version field, exclude-use-default → exclusions.presets, merged/removed linters, run.timeout behavior)
- https://golangci-lint.run/docs/configuration/file/ — v2 config file schema (top-level keys, linters.default values, exclusions structure)
- https://golangci-lint.run/docs/welcome/quick-start/ — confirmed the 5 linters in the v2 "standard" default set

### Repo files verified
- `.golangci.yml` (18 lines, v1 format as described)
- `.github/workflows/test.yml` line 30–33 — uses `golangci/golangci-lint-action@v7` + `version: latest` (correct for v2)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|---|---|---|
| A1 | No net-new findings after migration (gosimple checks already ran via merge) | Default-Linter Behavior Change Risk | Low — CI run will surface any surprises; fix by adding `disable:` entries |

## Metadata

- **Confidence:** HIGH across the board. Official docs give exact v1→v2 mapping for every line in the current file.
- **Valid until:** 2026-05-11 (30 days — golangci-lint v2 schema is stable as of v2.11).

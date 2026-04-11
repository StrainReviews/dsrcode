---
phase: 260411-kcq-fix-golangci-lint-v2-config-migration-in
plan: 01
subsystem: ci
tags:
  - ci
  - golangci-lint
  - tooling
  - fix
requirements:
  - QUICK-260411-kcq
dependency-graph:
  requires: []
  provides:
    - golangci-lint v2 compatible config
  affects:
    - .github/workflows/test.yml (consumes the fixed config)
tech-stack:
  added: []
  patterns:
    - YAML config versioning via mandatory `version:` field
key-files:
  created: []
  modified:
    - .golangci.yml
decisions:
  - Used explicit `linters.default: none` + enable list form (not the shorter alternative) for self-documentation and future-proofing
  - Did not bump project version — this is a CI tooling fix, not a user-facing change
  - Did not touch `.github/workflows/test.yml` — `golangci-lint-action@v7` + `version: latest` is already v2-compatible
metrics:
  duration: ~3 minutes
  completed: 2026-04-11
  tasks: 1
  files: 1
  commits: 1
---

# Quick Task 260411-kcq: Fix golangci-lint v2 config migration Summary

Migrated `.golangci.yml` from v1 to v2 schema to unblock CI, which was failing with `unsupported version of the configuration: ""` after `golangci-lint-action@v7` began resolving `version: latest` to a v2 release (v2.11.4).

## What Changed

Single file, surgical diff (9 insertions, 8 deletions):

```diff
-# golangci-lint configuration for CI
+# golangci-lint v2 configuration for CI
 # Run: golangci-lint run ./...
-run:
-  timeout: 5m
+version: "2"

 linters:
+  default: none
   enable:
     - errcheck
     - govet
     - staticcheck
     - unused
     - ineffassign
-    - gosimple
-    - typecheck
-
-issues:
-  exclude-use-default: true
+  exclusions:
+    presets:
+      - comments
+      - common-false-positives
+      - legacy
+      - std-error-handling
```

### Rationale for each change

| Change | Why |
|---|---|
| Add `version: "2"` | Mandatory in v2; its absence is the exact CI error. Must be quoted string (int `2` is rejected). |
| Add `linters.default: none` | Preserves v1 `disable-all: true` intent — only the 5 explicitly enabled linters run. |
| Drop `gosimple` | Merged into `staticcheck` in v2; listing it produces an unknown-linter error. Its checks still run transitively via staticcheck. |
| Drop `typecheck` | No longer user-selectable in v2; always runs implicitly. |
| Drop `run.timeout: 5m` | v2 has no default timeout and ignores this key per the migration guide. |
| `issues.exclude-use-default: true` → `linters.exclusions.presets` | The 4 presets (comments, common-false-positives, legacy, std-error-handling) are the v2 equivalent of the v1 default-excludes set per the official migration guide. |

## Verification Results

All 6 automated checks from Task 1 `<verify>` block passed:

| # | Check | Result |
|---|---|---|
| 1 | `head -3` → line 3 is `version: "2"` | PASS |
| 2a | `grep -c gosimple` → 0 | PASS (0) |
| 2b | `grep -c typecheck` → 0 | PASS (0) |
| 3 | `grep -c exclude-use-default` → 0 | PASS (0) |
| 4 | `grep -c 'presets:'` → >= 1 | PASS (1) |
| 5 | 5 enabled linters grep → 5 | PASS (5) |
| 6 | YAML syntactic validity (`python -c yaml.safe_load`) | PASS (`YAML_OK`) |

Additional gates:
- `git diff --stat` shows exactly one file changed (`.golangci.yml`, +9/-8)
- `git diff` content matches RESEARCH.md drop-in block verbatim (comment wording updated to "v2")
- Commit made: `4e9c9dc` — `fix(ci): migrate .golangci.yml to v2 schema`
- File is 18 lines total (17 content + trailing newline); within spec

## Deviations from Plan

None — plan executed exactly as written. RESEARCH.md drop-in block used verbatim.

## Deferred / Out-of-Scope

- **Functional CI-green verification** is deferred to the next push/PR. Local `golangci-lint` is not installed, and per the plan this is not a task gate. The `unsupported version of the configuration: ""` error will be confirmed gone on the next CI run.
- **Net-new lint findings** — per RESEARCH.md § Default-Linter Behavior Change Risk, none are expected since the effective linter set is unchanged (staticcheck now transitively covers former gosimple checks). Any surprise findings will be handled in a follow-up task by adding targeted `linters.disable:` entries, not by reverting this migration.

## Known Stubs

None. No placeholder values, no TODO markers, no unwired components.

## Commits

| Hash | Message |
|---|---|
| `4e9c9dc` | `fix(ci): migrate .golangci.yml to v2 schema` |

## Self-Check: PASSED

- FOUND: `.golangci.yml` (modified, v2 schema, 18 lines)
- FOUND: commit `4e9c9dc` in `git log`
- FOUND: all 6 automated verification checks pass
- FOUND: diff matches expected shape (removed run.timeout, gosimple, typecheck, exclude-use-default; added version: "2", linters.default: none, linters.exclusions.presets)

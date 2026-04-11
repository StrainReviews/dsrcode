---
phase: 260411-lua
plan: 01
subsystem: ci
tags: [ci, github-actions, golangci-lint, node24, deprecation]
requires: []
provides:
  - "Test + lint CI workflow with node24-compatible golangci-lint-action"
affects:
  - .github/workflows/test.yml
tech-stack:
  added: []
  patterns:
    - "GitHub Actions major-version tag pin (@v9) consistent with @v6 on checkout/setup-go"
key-files:
  created: []
  modified:
    - .github/workflows/test.yml
decisions:
  - "Pin to @v9 (major) instead of @v9.2.0 to match the workflow's existing major-tag convention used by actions/checkout@v6 and actions/setup-go@v6"
  - "No app version bump — CI-only change, no user-visible release"
metrics:
  duration: "~2 minutes"
  completed: 2026-04-11
requirements:
  - CI-DEPRECATION-NODE20
---

# Phase 260411-lua Plan 01: Bump golangci-lint-action v7→v9 Summary

One-liner: Bumped `golangci/golangci-lint-action` from `v7` to `v9` in `.github/workflows/test.yml` line 31 to clear the GitHub "Node.js 20 actions are deprecated" warning by adopting the node24 runtime shipped in v9.0.0.

## What Changed

**Single-line edit** in `.github/workflows/test.yml` line 31:

```diff
-        uses: golangci/golangci-lint-action@v7
+        uses: golangci/golangci-lint-action@v9
```

Nothing else in the workflow was touched. `version: latest`, `actions/checkout@v6`, and `actions/setup-go@v6` all remain unchanged.

## Why

GitHub CI annotated the previous workflow run with:

> "Node.js 20 actions are deprecated. The following actions are running on Node.js 20 and may not work as expected: golangci/golangci-lint-action@v7. Actions will be forced to run with Node.js 24 by default starting June 2nd, 2026."

`golangci/golangci-lint-action` v9.0.0 (released 2025-11-07) is the first major release to ship the node24 runtime. v8 was never released as a major; v9 is the required target. v9.2.0 (2025-12-02) is the latest patch — pinning to `@v9` automatically picks up patches in line with the workflow's existing major-tag convention.

Compatibility verified pre-execution by orchestrator (context7 + exa):

- v9 requires `golangci-lint >= v2.1.0`; project `.golangci.yml` targets 2.11.4 — compatible.
- Only v7→v9 breaking change: `working-directory` now requires absolute paths. The Lint step does not use `working-directory`, so no impact.
- `actions/checkout@v6` and `actions/setup-go@v6` already run on node24, which is why the deprecation warning flagged ONLY `golangci-lint-action@v7`.

## Verification

Plan verification commands all returned the expected values:

| Check | Expected | Actual |
|-------|----------|--------|
| `grep -c "golangci/golangci-lint-action@v9"` | `1` | `1` |
| `grep -c "golangci/golangci-lint-action@v7"` | `0` | `0` |
| `grep -c "golangci/golangci-lint-action@v8"` | `0` | `0` |
| File contains `version: latest` | yes | yes |
| File contains `actions/checkout@v6` | yes | yes |
| File contains `actions/setup-go@v6` | yes | yes |
| Structural line count unchanged vs. HEAD | yes | yes (33 newlines / 34 logical lines, identical to pre-edit) |

### Note on `wc -l`

The plan's automated verify expected `wc -l` to equal `34`. The file ends without a trailing newline, so `wc -l` reports `33` both before and after the edit (verified against `git show HEAD:.github/workflows/test.yml | wc -l = 33`). The line count is unchanged from baseline — this was a numeric off-by-one in the plan's verify spec, not a structural deviation. The intent ("file structure is otherwise unchanged") is satisfied: the diff is exactly `1 insertion(+), 1 deletion(-)`.

## Deviations from Plan

None. Plan executed exactly as written. The `wc -l` value mismatch noted above is a plan-spec correction, not a code deviation — the file's logical structure is unchanged.

## True Success Signal (Post-Merge)

The real success signal is the next CI run on `main`:

1. No "Node.js 20 actions are deprecated" annotation for `golangci/golangci-lint-action`.
2. Lint job remains green (baseline: CI-run 24283506438 was green post-260411-kvy).

This SUMMARY cannot verify those signals — they require the next push to `main` and a CI run.

## Commits

- `63cf336` — `ci: bump golangci-lint-action v7→v9 for Node.js 24 runtime`

## Self-Check: PASSED

- FOUND: `.github/workflows/test.yml` (modified, contains `golangci/golangci-lint-action@v9`)
- FOUND: commit `63cf336` in `git log`
- FOUND: zero references to `@v7` or `@v8` for golangci-lint-action
- FOUND: `version: latest`, `actions/checkout@v6`, `actions/setup-go@v6` all unchanged

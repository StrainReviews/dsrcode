---
phase: 260411-lua
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .github/workflows/test.yml
autonomous: true
requirements:
  - CI-DEPRECATION-NODE20
must_haves:
  truths:
    - "CI test workflow uses golangci-lint-action@v9 (Node.js 24 runtime)"
    - "No reference to golangci-lint-action@v7 remains in the workflow"
    - "No accidental v8 pin introduced"
    - "Workflow structure is otherwise unchanged (same line count, same steps)"
    - "Lint step still runs with version: latest"
  artifacts:
    - path: ".github/workflows/test.yml"
      provides: "Test + lint CI workflow with node24-compatible golangci-lint-action"
      contains: "golangci/golangci-lint-action@v9"
  key_links:
    - from: ".github/workflows/test.yml line 31"
      to: "golangci/golangci-lint-action@v9 runtime"
      via: "GitHub Actions uses: directive"
      pattern: "golangci/golangci-lint-action@v9"
---

<objective>
Bump `golangci/golangci-lint-action` from `v7` to `v9` in the CI test workflow to eliminate the GitHub "Node.js 20 actions are deprecated" warning. v9.0.0 (2025-11-07) is the first major release shipping the node24 runtime, which is required before the June 2, 2026 forced-migration date.

Purpose: Clear the deprecation warning on the last CI run and future-proof the workflow against the June 2026 node24 enforcement. All other actions in the workflow (`actions/checkout@v6`, `actions/setup-go@v6`) already run on node24, so this is the only change needed.

Output: Updated `.github/workflows/test.yml` with a single character change on line 31 (`v7` → `v9`).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
</execution_context>

<context>
@.github/workflows/test.yml
@CLAUDE.md

<interfaces>
<!-- Current state of the edit site (lines 30-33 of .github/workflows/test.yml) -->

```yaml
      - name: Lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: latest
```

After edit:
```yaml
      - name: Lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: latest
```

Nothing else in the workflow changes. The file is 34 lines total and should remain 34 lines after the edit.
</interfaces>

<mcp_findings>
Orchestrator already MCP-verified via context7 + exa:

- **v9.0.0 release notes:** "requires Nodejs runtime node24" (first major version to drop node20)
- **v9.2.0** (2025-12-02) is the latest patch; pinning to `@v9` auto-picks up patches per the workflow's existing major-tag convention (matches `@v6` for checkout/setup-go)
- **Compatibility:** v9 requires `golangci-lint >= v2.1.0`; our `.golangci.yml` targets 2.11.4 — compatible
- **Breaking changes v7 → v9:** Only `working-directory` now requires absolute paths. We do not use `working-directory`, so no impact.
- **Other actions:** `actions/checkout@v6` and `actions/setup-go@v6` already run on node24 — that is why the last CI run flagged ONLY `golangci-lint-action@v7` in the deprecation warning, not the other steps.
- **CI green baseline:** Lint job passed on CI-run 24283506438 post-260411-kvy, so we know the lint config is already correct — this change only swaps the runner wrapper.

Do NOT re-research.
</mcp_findings>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Bump golangci-lint-action v7 to v9</name>
  <files>.github/workflows/test.yml</files>
  <action>
Edit `.github/workflows/test.yml` line 31. Change exactly one token:

- Before: `        uses: golangci/golangci-lint-action@v7`
- After:  `        uses: golangci/golangci-lint-action@v9`

Do NOT:
- Change `actions/checkout@v6` (already node24)
- Change `actions/setup-go@v6` (already node24)
- Change the `with:` block — `version: latest` stays exactly as is
- Pin to `@v9.2.0` — the workflow uses major-version tags (see `@v6` on checkout/setup-go), so match that convention with `@v9`
- Drop to `@v8` by mistake — v9 is the required target (v8 may not exist as a major release; v9.0.0 is the first node24 release)
- Touch `.golangci.yml` — lint config is correct post-260411-kcq
- Bump the app version in `main.go` / `plugin.json` / `marketplace.json` — this is a CI-only change, no user-visible release

Rationale (for the commit body): GitHub CI annotation on the last run warned "Node.js 20 actions are deprecated. The following actions are running on Node.js 20 and may not work as expected: golangci/golangci-lint-action@v7. Actions will be forced to run with Node.js 24 by default starting June 2nd, 2026." golangci/golangci-lint-action v9.0.0 (released 2025-11-07) is the first major release to switch to the node24 runtime. Verified compatible with our golangci-lint 2.11.4 config and our workflow (we do not use `working-directory`, the only breaking change).
  </action>
  <verify>
    <automated>bash -c 'set -e; f=.github/workflows/test.yml; test "$(grep -c "golangci/golangci-lint-action@v9" "$f")" = "1" && test "$(grep -c "golangci/golangci-lint-action@v7" "$f")" = "0" && test "$(grep -c "golangci/golangci-lint-action@v8" "$f")" = "0" && test "$(wc -l < "$f")" = "34" && grep -q "version: latest" "$f" && grep -q "actions/checkout@v6" "$f" && grep -q "actions/setup-go@v6" "$f"'</automated>
  </verify>
  <done>
- `.github/workflows/test.yml` line 31 reads `uses: golangci/golangci-lint-action@v9`
- Zero references to `@v7` or `@v8` for golangci-lint-action remain
- File is still 34 lines long (no structural changes)
- `version: latest` is unchanged in the Lint step
- `actions/checkout@v6` and `actions/setup-go@v6` are unchanged
- No other files modified
- True verification (post-merge): the next CI run shows no "Node.js 20 actions are deprecated" warning for golangci-lint-action, and the Lint job remains green (baseline: CI-run 24283506438 was green post-260411-kvy)
  </done>
</task>

</tasks>

<verification>
Automated:
1. `grep -c "golangci/golangci-lint-action@v9" .github/workflows/test.yml` → `1`
2. `grep -c "golangci/golangci-lint-action@v7" .github/workflows/test.yml` → `0`
3. `grep -c "golangci/golangci-lint-action@v8" .github/workflows/test.yml` → `0`
4. `wc -l .github/workflows/test.yml` → `34`
5. File still contains `version: latest`, `actions/checkout@v6`, `actions/setup-go@v6`

Post-merge (outside this plan's scope but is the real success signal):
- Next CI run on `main` shows no Node.js 20 deprecation annotation
- Lint job passes (green)
</verification>

<success_criteria>
- One-character edit applied cleanly to line 31 of `.github/workflows/test.yml`
- All `grep`/`wc` verification commands return expected values
- No other files touched in the repository
- Commit uses subject: `ci: bump golangci-lint-action v7→v9 for Node.js 24 runtime`
- Commit body references the GitHub deprecation warning and v9.0.0 node24 migration
- No app version bump (CI-only change)
</success_criteria>

<output>
After completion, create `.planning/quick/260411-lua-bump-golangci-lint-action-v7-to-v9-for-n/260411-lua-01-SUMMARY.md` documenting:
- The single-line edit made
- Confirmation all verify commands passed
- Reminder that the true success signal is the next CI run (no deprecation warning + green lint)
</output>

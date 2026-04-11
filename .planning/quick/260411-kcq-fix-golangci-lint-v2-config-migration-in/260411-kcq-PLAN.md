---
phase: 260411-kcq-fix-golangci-lint-v2-config-migration-in
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .golangci.yml
autonomous: true
requirements:
  - QUICK-260411-kcq
tags:
  - ci
  - golangci-lint
  - tooling

must_haves:
  truths:
    - "`.golangci.yml` is parsed successfully by golangci-lint v2 (no `unsupported version of the configuration: \"\"` error)"
    - "The 5 linters the project cares about (errcheck, govet, staticcheck, unused, ineffassign) are enabled"
    - "No removed/merged v1 linters (`gosimple`, `typecheck`) appear in the file"
    - "The v1 `issues.exclude-use-default: true` semantics are preserved via v2 `linters.exclusions.presets`"
    - "CI (`.github/workflows/test.yml`, `golangci-lint-action@v7` + `version: latest`) runs lint to completion on next push"
  artifacts:
    - path: ".golangci.yml"
      provides: "golangci-lint v2 config"
      contains: "version: \"2\""
      min_lines: 15
  key_links:
    - from: ".golangci.yml"
      to: "golangci-lint v2 schema"
      via: "top-level `version: \"2\"` field"
      pattern: "^version: \"2\"$"
    - from: ".golangci.yml"
      to: "exclusion presets (replaces v1 exclude-use-default)"
      via: "linters.exclusions.presets block"
      pattern: "presets:"
---

<objective>
Replace the v1-format `.golangci.yml` with a v2-format drop-in equivalent so CI (`golangci-lint-action@v7`, `version: latest` → v2.11.4) stops failing with `unsupported version of the configuration: ""`.

Purpose: Unblock CI. The current file is missing the now-mandatory `version: "2"` header, lists two linters that no longer exist as user-selectable in v2 (`gosimple` merged into `staticcheck`, `typecheck` no longer selectable), and uses the obsolete `issues.exclude-use-default: true` key.

Output: A 15-line v2 `.golangci.yml` that preserves v1 intent (same 5 effective linters, same default-exclusion behavior) with zero net-new findings expected.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/quick/260411-kcq-fix-golangci-lint-v2-config-migration-in/260411-kcq-RESEARCH.md
@.golangci.yml
@.github/workflows/test.yml

<notes>
RESEARCH.md has been MCP-verified by the orchestrator:
- context7 (`/websites/golangci-lint_run`) confirmed v2 config schema (`version: "2"`, `linters.default: none`, `linters.exclusions.presets`)
- crawl4ai fetched https://golangci-lint.run/product/migration-guide/ — authoritative migration guide
- exa confirmed `version` field is mandatory and all migration mappings

Plan with HIGH confidence. Use the "Recommended v2 `.golangci.yml` (drop-in replacement)" block from RESEARCH.md lines 23–42 **verbatim** (explicit `default: none` + enable list form — NOT the shorter alternative). Only the leading comment wording is updated to reflect v2.
</notes>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Rewrite .golangci.yml to v2 schema (drop-in)</name>
  <files>.golangci.yml</files>
  <action>
Overwrite `.golangci.yml` with exactly the following 15-line content (verbatim from RESEARCH.md "Recommended v2 `.golangci.yml` (drop-in replacement)" block, lines 23–42, with only the leading comment wording updated to say "v2"). Do NOT use the shorter alternative block from RESEARCH.md line 57 — we want the explicit form for self-documentation and future-proofing:

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

Specifics (why each change, from RESEARCH.md):
- `version: "2"` is mandatory in v2; its absence is the exact CI error. It MUST be a quoted string — `version: 2` parses as int and is rejected.
- `linters.default: none` + explicit `enable:` preserves v1 intent (`disable-all: true` equivalent). Do NOT use v2 "standard" defaults implicitly.
- Drop `gosimple` — merged into `staticcheck` in v2; listing it produces an "unknown linter" error.
- Drop `typecheck` — no longer user-selectable in v2; always runs implicitly. Listing it errors out.
- Drop `run.timeout: 5m` — v2 has no timeout by default and the migration guide explicitly says the existing value is ignored. Removing keeps the file clean.
- Convert `issues.exclude-use-default: true` → `linters.exclusions.presets: [comments, common-false-positives, legacy, std-error-handling]` — these 4 presets are the v2 equivalent of the v1 "default excludes" set per the migration guide.

Do NOT touch `.github/workflows/test.yml` — per RESEARCH.md § Pitfall 6, it already pins `golangci/golangci-lint-action@v7` with `version: latest`, which is v2-compatible.

Do NOT bump the project version (main.go, plugin.json, marketplace.json, start.sh, start.ps1) — this is a CI tooling fix, not a user-facing feature or release.

After writing, the file must be exactly 15 non-trailing lines (2 comment + 1 blank + 12 config). Use the Write tool to create the file (do not use heredoc/cat).
  </action>
  <verify>
    <automated>
Run these checks in a single bash invocation and confirm ALL pass:

1. Header present and correctly quoted:
   `head -3 .golangci.yml` → line 3 must be exactly `version: "2"`

2. Removed linters absent:
   `grep -c gosimple .golangci.yml` → must return `0`
   `grep -c typecheck .golangci.yml` → must return `0`

3. Obsolete v1 exclusion key removed:
   `grep -c 'exclude-use-default' .golangci.yml` → must return `0`

4. v2 exclusion presets block present:
   `grep -c 'presets:' .golangci.yml` → must return `>= 1`

5. All 5 intended linters still enabled:
   `grep -E '^\s+- (errcheck|govet|staticcheck|unused|ineffassign)$' .golangci.yml | wc -l` → must return `5`

6. YAML syntactic validity — run whichever is available:
   - If `python` is on PATH: `python -c "import yaml,sys; yaml.safe_load(open('.golangci.yml'))" && echo YAML_OK`
   - Else if `node` is on PATH and `js-yaml` importable: `node -e "require('js-yaml').load(require('fs').readFileSync('.golangci.yml','utf8'))" && echo YAML_OK`
   - If neither is available: skip with a note in the summary (YAML validity will be re-checked by golangci-lint itself on next CI run).

Note: Functional CI-green verification (actual `golangci-lint run` against the repo on GitHub Actions) is OUT OF SCOPE for this local task — it happens on the next push/PR. Local `golangci-lint` is not assumed to be installed. If a user wants to pre-validate locally before pushing, they may run `golangci-lint config verify` — but this is not a task gate.
    </automated>
  </verify>
  <done>
`.golangci.yml` is a 15-line v2-schema file containing `version: "2"`, explicit `linters.default: none` + 5-entry `enable:` list (errcheck, govet, staticcheck, unused, ineffassign), and a 4-preset `linters.exclusions.presets` block. All 6 automated checks above pass. No `gosimple`, `typecheck`, `exclude-use-default`, or `run.timeout` strings remain in the file. No other files modified.
  </done>
</task>

</tasks>

<verification>
Local gates (this plan):
- All 6 automated checks in Task 1 `<verify>` pass.
- `git diff --stat` shows exactly one file changed: `.golangci.yml`.
- `git diff .golangci.yml` shows removal of `run.timeout`, `gosimple`, `typecheck`, `issues.exclude-use-default`, and addition of `version: "2"`, `linters.default: none`, `linters.exclusions.presets`.

Deferred gate (outside this plan):
- Real functional verification is the next CI run on push/PR. The `unsupported version of the configuration: ""` error must be gone and `golangci-lint run ./...` must complete. Per RESEARCH.md § Default-Linter Behavior Change Risk, no net-new findings are expected since the v2 standard default set equals the project's intended set and `gosimple`'s checks were already running transitively. Any surprise findings should be handled in a follow-up task (add targeted `linters.disable:` entries), not by reverting this migration.
</verification>

<success_criteria>
- CI no longer fails with `unsupported version of the configuration: ""` on the next push.
- The same 5 effective linters run (errcheck, govet, staticcheck, unused, ineffassign; staticcheck now transitively covers former gosimple checks).
- v1 "default excludes" behavior is preserved via v2 `exclusions.presets`.
- No workflow file, no source file, no version bump — surgical one-file diff.
</success_criteria>

<output>
After completion, create `.planning/quick/260411-kcq-fix-golangci-lint-v2-config-migration-in/260411-kcq-SUMMARY.md` per the summary template, recording: file diff size, verification results, and a note that true CI-green confirmation is deferred to the next push.
</output>

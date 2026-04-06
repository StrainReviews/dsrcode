#!/usr/bin/env bash
# DSR Code Presence — install skills to ~/.claude/skills/ for autocomplete
# Runs on SessionStart via plugin hooks. Idempotent — skips if already current.

set -euo pipefail

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
SKILLS_DIR="$HOME/.claude/skills"
VERSION_FILE="$HOME/.claude/.dsrcode-skills-version"
CURRENT_VERSION=$(grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$PLUGIN_ROOT/.claude-plugin/plugin.json" 2>/dev/null | head -1 | sed 's/.*"version"[[:space:]]*:[[:space:]]*"//' | sed 's/"//')

if [ -z "$CURRENT_VERSION" ]; then
  exit 0
fi

# Check if already installed at this version
if [ -f "$VERSION_FILE" ] && [ "$(cat "$VERSION_FILE" 2>/dev/null)" = "$CURRENT_VERSION" ]; then
  exit 0
fi

# Install/update skills
mkdir -p "$SKILLS_DIR"

for skill_dir in "$PLUGIN_ROOT"/skills/*/; do
  skill_name=$(basename "$skill_dir")
  target_dir="$SKILLS_DIR/dsrcode-$skill_name"

  # Remove old version if exists
  rm -rf "$target_dir"

  # Copy skill directory
  mkdir -p "$target_dir"
  cp -r "$skill_dir"* "$target_dir/" 2>/dev/null || true
done

# Write version marker
echo "$CURRENT_VERSION" > "$VERSION_FILE"

echo "dsrcode v$CURRENT_VERSION: 7 skills installed to $SKILLS_DIR"

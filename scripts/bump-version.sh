#!/bin/bash
# Bump version across all files that contain version numbers.
# Usage: ./scripts/bump-version.sh 4.1.0
# Updates: main.go, plugin.json, marketplace.json, start.sh, start.ps1 (5 files total)
# Does NOT auto-commit. Prints git instructions after updating.

set -e

if [[ -z "$1" ]]; then
    echo "Usage: $0 <version>" >&2
    echo "Example: $0 4.1.0" >&2
    exit 1
fi

NEW_VERSION="$1"
# Strip 'v' prefix if provided (e.g., v4.1.0 -> 4.1.0)
NEW_VERSION="${NEW_VERSION#v}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Bumping version to ${NEW_VERSION}..."

# 1. main.go: var version = "dev" -> var version = "X.Y.Z"
#    Updates the fallback version shown in dev builds without GoReleaser.
#    GoReleaser still injects the real version via ldflags at release build time.
sed -i.bak "s/var version = \"[^\"]*\"/var version = \"${NEW_VERSION}\"/" "$REPO_ROOT/main.go"
rm -f "$REPO_ROOT/main.go.bak"
echo "  Updated main.go"

# 2. plugin.json: "version": "X.Y.Z"
sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW_VERSION}\"/" "$REPO_ROOT/.claude-plugin/plugin.json"
rm -f "$REPO_ROOT/.claude-plugin/plugin.json.bak"
echo "  Updated .claude-plugin/plugin.json"

# 3. marketplace.json: "version": "X.Y.Z"
sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW_VERSION}\"/" "$REPO_ROOT/.claude-plugin/marketplace.json"
rm -f "$REPO_ROOT/.claude-plugin/marketplace.json.bak"
echo "  Updated .claude-plugin/marketplace.json"

# 4. start.sh: VERSION="vX.Y.Z"
sed -i.bak "s/VERSION=\"v[^\"]*\"/VERSION=\"v${NEW_VERSION}\"/" "$REPO_ROOT/scripts/start.sh"
rm -f "$REPO_ROOT/scripts/start.sh.bak"
echo "  Updated scripts/start.sh"

# 5. start.ps1: $Version = "vX.Y.Z"
sed -i.bak "s/\\\$Version = \"v[^\"]*\"/\$Version = \"v${NEW_VERSION}\"/" "$REPO_ROOT/scripts/start.ps1"
rm -f "$REPO_ROOT/scripts/start.ps1.bak"
echo "  Updated scripts/start.ps1"

echo ""
echo "Version bumped to ${NEW_VERSION} in 5 files."
echo ""
echo "Next steps:"
echo "  git add main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1"
echo "  git commit -m \"chore: bump version to v${NEW_VERSION}\""
echo "  git tag v${NEW_VERSION}"
echo "  git push origin main --tags"
echo ""
echo "The tag push triggers the release workflow automatically."

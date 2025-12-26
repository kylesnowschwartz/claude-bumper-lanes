#!/bin/bash
# ensure-binaries.sh - Idempotent binary builder for bumper-lanes
# Called by SessionStart hook. Detects stale binaries via version sentinel.
#
# Version sentinel strategy:
# - plugin.json version is the source of truth (works in both dev and cached installs)
# - bin/.build-version stores the version that was built
# - If mismatch or missing binaries → rebuild
# - If match → fast-path exit (~15ms)
set -e

BIN="${CLAUDE_PLUGIN_ROOT}/bin"
VERSION_FILE="$BIN/.build-version"
PLUGIN_JSON="${CLAUDE_PLUGIN_ROOT}/.claude-plugin/plugin.json"
mkdir -p "$BIN"

# Get current plugin version (works in dev and cached installs)
if [ -f "$PLUGIN_JSON" ]; then
  CURRENT_VERSION=$(grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$PLUGIN_JSON" | sed 's/.*: *"\([^"]*\)"/\1/')
else
  CURRENT_VERSION="unknown"
fi

# Get built version from sentinel
BUILT_VERSION=$(cat "$VERSION_FILE" 2>/dev/null || echo "none")

# Fast path: version matches and binaries exist (~15ms)
if [ "$CURRENT_VERSION" = "$BUILT_VERSION" ] &&
  [ -x "$BIN/bumper-lanes" ] &&
  [ -x "$BIN/git-diff-tree" ]; then
  exit 0
fi

# Version mismatch or missing binaries - rebuild
if [ "$BUILT_VERSION" != "none" ] && [ "$CURRENT_VERSION" != "$BUILT_VERSION" ]; then
  echo "Bumper-lanes updated ($BUILT_VERSION → $CURRENT_VERSION), rebuilding..." >&2
elif [ "$BUILT_VERSION" = "none" ]; then
  echo "Bumper-lanes first install, building binaries..." >&2
else
  echo "Bumper-lanes binaries missing, rebuilding..." >&2
fi

# Build bumper-lanes
echo "  Building bumper-lanes..." >&2
cd "${CLAUDE_PLUGIN_ROOT}/tools/bumper-lanes"
go build -o "$BIN/bumper-lanes" ./cmd/bumper-lanes
echo "  Built: bumper-lanes" >&2

# Install git-diff-tree
if [ ! -x "$BIN/git-diff-tree" ]; then
  echo "  Installing git-diff-tree..." >&2
  go install github.com/kylesnowschwartz/diff-viz/cmd/git-diff-tree@latest
  GOBIN="$(go env GOPATH)/bin"
  if [ -x "$GOBIN/git-diff-tree" ]; then
    cp "$GOBIN/git-diff-tree" "$BIN/"
    echo "  Installed: git-diff-tree" >&2
  else
    echo "Warning: git-diff-tree not found in $GOBIN" >&2
    exit 1
  fi
fi

# Write version sentinel
echo "$CURRENT_VERSION" >"$VERSION_FILE"
echo "Bumper-lanes ready (v$CURRENT_VERSION)" >&2

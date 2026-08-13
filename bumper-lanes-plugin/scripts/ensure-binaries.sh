#!/bin/bash
# ensure-binaries.sh - Idempotent binary builder for bumper-lanes
# Called by SessionStart hook. Detects stale binaries via version sentinel.
#
# Version sentinel strategy:
# - plugin.json version is the source of truth (works in both dev and cached installs)
# - bin/.build-version stores the version that was built
# - If mismatch or missing binary → rebuild
# - If match → fast-path exit (~15ms)
#
# Note: diff-viz is now a library dependency (via go.mod), not a separate binary.
# Version updates happen through `go get github.com/kylesnowschwartz/diff-viz@latest`.
set -e

BIN="${CLAUDE_PLUGIN_ROOT}/bin"
VERSION_FILE="$BIN/.build-version"
PLUGIN_JSON="${CLAUDE_PLUGIN_ROOT}/.claude-plugin/plugin.json"
mkdir -p "$BIN"

# Build-progress messages go to the install log, not stderr: Claude Code
# labels any SessionStart stderr as a "hook error" even when the hook
# succeeds, which reads as a failed install.
LOG_DIR="$HOME/.claude/logs/bumper-lanes"
mkdir -p "$LOG_DIR"
INSTALL_LOG="$LOG_DIR/$(date +%F)-install.log"
log() { echo "[$(date '+%F %T')] $*" >> "$INSTALL_LOG"; }

# Get current plugin version (works in dev and cached installs)
if [ -f "$PLUGIN_JSON" ]; then
  CURRENT_VERSION=$(grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$PLUGIN_JSON" | sed 's/.*: *"\([^"]*\)"/\1/')
else
  CURRENT_VERSION="unknown"
fi

# Get built version from sentinel
BUILT_VERSION=$(cat "$VERSION_FILE" 2>/dev/null || echo "none")

# Fast path: version matches and binary exists (~15ms)
if [ "$CURRENT_VERSION" = "$BUILT_VERSION" ] && [ -x "$BIN/bumper-lanes" ]; then
  exit 0
fi

# Version mismatch or missing binary - rebuild
if [ "$BUILT_VERSION" != "none" ] && [ "$CURRENT_VERSION" != "$BUILT_VERSION" ]; then
  log "Bumper-lanes updated ($BUILT_VERSION → $CURRENT_VERSION), rebuilding..."
elif [ "$BUILT_VERSION" = "none" ]; then
  log "Bumper-lanes first install, building binary..."
else
  log "Bumper-lanes binary missing, rebuilding..."
fi

# Build bumper-lanes (includes diff-viz as library dependency)
log "  Building bumper-lanes..."
cd "${CLAUDE_PLUGIN_ROOT}/tools/bumper-lanes"
go build -o "$BIN/bumper-lanes" ./cmd/bumper-lanes
log "  Built: bumper-lanes"

# Write version sentinel
echo "$CURRENT_VERSION" >"$VERSION_FILE"
log "Bumper-lanes ready (v$CURRENT_VERSION)"

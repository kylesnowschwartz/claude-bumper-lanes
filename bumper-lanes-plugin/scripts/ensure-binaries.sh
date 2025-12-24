#!/bin/bash
# ensure-binaries.sh - Idempotent binary builder for bumper-lanes
# Called by SessionStart hook. Fast-path if binaries exist, builds otherwise.
set -e

BIN="${CLAUDE_PLUGIN_ROOT}/bin"
mkdir -p "$BIN"

# Fast path: both binaries exist and are executable (~5ms)
[ -x "$BIN/bumper-lanes" ] && [ -x "$BIN/git-diff-tree" ] && exit 0

# Slow path: build/install missing binaries (one-time, ~10-15s)
if [ ! -x "$BIN/bumper-lanes" ]; then
  echo "Building bumper-lanes (one-time)..." >&2
  cd "${CLAUDE_PLUGIN_ROOT}/tools/bumper-lanes"
  go build -o "$BIN/bumper-lanes" ./cmd/bumper-lanes
  echo "Built: $BIN/bumper-lanes" >&2
fi

if [ ! -x "$BIN/git-diff-tree" ]; then
  echo "Installing git-diff-tree (one-time)..." >&2
  go install github.com/kylesnowschwartz/diff-viz/cmd/git-diff-tree@latest
  GOBIN="$(go env GOPATH)/bin"
  if [ -x "$GOBIN/git-diff-tree" ]; then
    cp "$GOBIN/git-diff-tree" "$BIN/"
    echo "Installed: $BIN/git-diff-tree" >&2
  else
    echo "Warning: git-diff-tree not found in $GOBIN" >&2
    exit 1
  fi
fi

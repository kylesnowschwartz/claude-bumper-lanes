#!/usr/bin/env bash
set -euo pipefail

# session-start.sh - SessionStart hook for baseline capture
# Purpose: Capture working tree state as baseline when Claude session starts

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id')

# Hook is already executed in project directory (cwd field in JSON)

# Check if this is a git repository
if ! git rev-parse --git-dir &>/dev/null; then
  # Not a git repo - disable plugin gracefully
  exit 0
fi

# Capture baseline tree
baseline_tree=$(capture_tree)
if [[ -z "$baseline_tree" ]]; then
  echo "ERROR: Failed to capture baseline tree" >&2
  exit 0 # Fail open
fi

# Capture current branch name for staleness detection
baseline_branch=$(get_current_branch)

# Write session state (with branch tracking)
write_session_state "$session_id" "$baseline_tree" "$baseline_branch"

# Allow session start
exit 0

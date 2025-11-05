#!/usr/bin/env bash
set -euo pipefail

# state-manager.sh - SessionId-based session state management
# Purpose: Persist and retrieve session state for threshold tracking

# write_session_state() - Writes session state JSON to .git/bumper-checkpoints/
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - baseline_tree (40-char git tree SHA)
# Creates: .git/bumper-checkpoints/session-{sessionId} with JSON state
write_session_state() {
  local session_id=$1
  local baseline_tree=$2
  local repo_path
  repo_path=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
  local checkpoint_dir=".git/bumper-checkpoints"

  mkdir -p "$checkpoint_dir" 2>/dev/null || true

  local state_file="$checkpoint_dir/session-$session_id"
  local timestamp
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  # Preserve stop_triggered flag if it exists, otherwise default to false
  local stop_triggered="false"
  if [[ -f "$state_file" ]]; then
    stop_triggered=$(jq -r '.stop_triggered // false' "$state_file" 2>/dev/null || echo "false")
  fi

  # generous threshold limit of 4000 was selected in order to allow for
  # live testing without onerous limits to start
  cat >"$state_file" <<EOF
{
  "session_id": "$session_id",
  "baseline_tree": "$baseline_tree",
  "created_at": "$timestamp",
  "threshold_limit": 400,
  "repo_path": "$repo_path",
  "stop_triggered": $stop_triggered
}
EOF

  return 0
}

# read_session_state() - Reads session state JSON from .git/bumper-checkpoints/
# Args:
#   $1 - session_id (conversation UUID)
# Returns: Session state JSON on stdout
# Returns 1 if state file doesn't exist
read_session_state() {
  local session_id=$1
  local checkpoint_dir=".git/bumper-checkpoints"
  local state_file="$checkpoint_dir/session-$session_id"

  if [[ ! -f "$state_file" ]]; then
    echo "ERROR: No session state found for session $session_id" >&2
    return 1
  fi

  cat "$state_file"
  return 0
}

# set_stop_triggered() - Set stop_triggered flag in session state
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - stop_triggered value (true|false)
# Updates: .git/bumper-checkpoints/session-{sessionId} with new flag value
set_stop_triggered() {
  local session_id=$1
  local stop_triggered=$2
  local checkpoint_dir=".git/bumper-checkpoints"
  local state_file="$checkpoint_dir/session-$session_id"

  if [[ ! -f "$state_file" ]]; then
    echo "ERROR: No session state found for session $session_id" >&2
    return 1
  fi

  # Use jq to update the stop_triggered field
  local temp_file
  temp_file=$(mktemp)
  jq --argjson stop_triggered "$stop_triggered" '.stop_triggered = $stop_triggered' "$state_file" >"$temp_file"
  mv "$temp_file" "$state_file"

  return 0
}

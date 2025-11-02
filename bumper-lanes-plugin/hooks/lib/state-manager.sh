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

  cat >"$state_file" <<EOF
{
  "session_id": "$session_id",
  "baseline_tree": "$baseline_tree",
  "created_at": "$timestamp",
  "threshold_limit": 300,
  "repo_path": "$repo_path"
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

#!/usr/bin/env bash
set -euo pipefail

# session-end.sh - SessionEnd hook for checkpoint cleanup
# Purpose: Remove session-specific checkpoint file when Claude session terminates

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id // ""')

# Validate session_id
if [[ -z "$session_id" ]]; then
  echo "ERROR: No session_id provided to SessionEnd hook" >&2
  exit 0 # Fail open - SessionEnd can't block anyway
fi

# Check if git repo (checkpoint dir won't exist otherwise)
git_dir=$(git rev-parse --git-dir 2>/dev/null) || exit 0

checkpoint_dir="$git_dir/bumper-checkpoints"
state_file="$checkpoint_dir/session-$session_id"

# Remove this session's checkpoint file
if [[ -f "$state_file" ]]; then
  rm -f "$state_file" 2>/dev/null || true
fi

# Optional: Clean stale checkpoints (files older than 30 days)
# Uncomment the line below to enable automatic stale file cleanup:
# find "$checkpoint_dir" -type f -name "session-*" -mtime +30 -delete 2>/dev/null || true

exit 0

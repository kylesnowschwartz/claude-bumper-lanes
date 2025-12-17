#!/usr/bin/env bash
set -euo pipefail

# state-manager.sh - SessionId-based session state management
# Purpose: Persist and retrieve session state for threshold tracking

# get_checkpoint_dir() - Returns path to checkpoint directory
# Handles git worktrees where .git is a file, not a directory
# Returns: Path like ".git/bumper-checkpoints" or "/path/.git/worktrees/name/bumper-checkpoints"
get_checkpoint_dir() {
  local git_dir
  git_dir=$(git rev-parse --git-dir 2>/dev/null) || return 1
  echo "$git_dir/bumper-checkpoints"
}

# write_session_state() - Writes session state JSON to bumper-checkpoints/
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - baseline_tree (40-char git tree SHA)
#   $3 - baseline_branch (optional branch name for staleness detection)
# Creates: {git-dir}/bumper-checkpoints/session-{sessionId} with JSON state
write_session_state() {
  local session_id=$1
  local baseline_tree=$2
  local baseline_branch=${3:-""}
  local repo_path
  repo_path=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
  local checkpoint_dir
  checkpoint_dir=$(get_checkpoint_dir) || return 1

  mkdir -p "$checkpoint_dir" 2>/dev/null || true

  local state_file="$checkpoint_dir/session-$session_id"
  local timestamp
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  # Preserve stop_triggered flag if it exists, otherwise default to false
  local stop_triggered="false"
  # Preserve accumulated_score and previous_tree for incremental tracking
  local accumulated_score="0"
  local previous_tree="$baseline_tree"

  if [[ -f "$state_file" ]]; then
    stop_triggered=$(jq -r '.stop_triggered // false' "$state_file" 2>/dev/null || echo "false")
    accumulated_score=$(jq -r '.accumulated_score // 0' "$state_file" 2>/dev/null || echo "0")
    previous_tree=$(jq -r '.previous_tree // .baseline_tree' "$state_file" 2>/dev/null || echo "$baseline_tree")
    # Preserve baseline_branch if not provided
    if [[ -z "$baseline_branch" ]]; then
      baseline_branch=$(jq -r '.baseline_branch // ""' "$state_file" 2>/dev/null || echo "")
    fi
  fi

  # WHY 400 points:
  # - Alpha testing value (generous to avoid hitting limits constantly)
  # - Production default should be 200 (GitLab-aligned, Cisco-validated)
  # - User can override by editing this file and running /bumper-reset
  #
  # Atomic write: temp file + mv to prevent race conditions when multiple
  # hooks run in parallel (e.g., 10 Stop hooks all firing simultaneously)
  local temp_file
  temp_file=$(mktemp)
  cat >"$temp_file" <<EOF
{
  "session_id": "$session_id",
  "baseline_tree": "$baseline_tree",
  "baseline_branch": "$baseline_branch",
  "previous_tree": "$previous_tree",
  "accumulated_score": $accumulated_score,
  "created_at": "$timestamp",
  "threshold_limit": 400,
  "repo_path": "$repo_path",
  "stop_triggered": $stop_triggered
}
EOF
  mv "$temp_file" "$state_file"

  return 0
}

# read_session_state() - Reads session state JSON from bumper-checkpoints/
# Args:
#   $1 - session_id (conversation UUID)
# Returns: Session state JSON on stdout
# Returns 1 if state file doesn't exist
read_session_state() {
  local session_id=$1
  local checkpoint_dir
  checkpoint_dir=$(get_checkpoint_dir) || return 1
  local state_file="$checkpoint_dir/session-$session_id"

  if [[ ! -f "$state_file" ]]; then
    echo "ERROR: No session state found for session $session_id" >&2
    return 1
  fi

  cat "$state_file"
  return 0
}

# _atomic_update_state() - Internal helper for atomic JSON state updates
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - jq filter expression (e.g., '.stop_triggered = $stop_triggered')
#   $3+ - jq arguments (e.g., '--argjson stop_triggered true')
# Returns: 0 on success, 1 if state file doesn't exist
# WHY: Eliminates duplicated temp file + validation pattern across 4 functions
_atomic_update_state() {
  local session_id=$1
  shift
  local jq_filter=$1
  shift
  local checkpoint_dir
  checkpoint_dir=$(get_checkpoint_dir) || return 1
  local state_file="$checkpoint_dir/session-$session_id"

  if [[ ! -f "$state_file" ]]; then
    echo "ERROR: No session state found for session $session_id" >&2
    return 1
  fi

  local temp_file
  temp_file=$(mktemp)
  jq "$@" "$jq_filter" "$state_file" >"$temp_file"
  mv "$temp_file" "$state_file"

  return 0
}

# set_stop_triggered() - Set stop_triggered flag in session state
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - stop_triggered value (true|false)
# Updates: {git-dir}/bumper-checkpoints/session-{sessionId} with new flag value
set_stop_triggered() {
  local session_id=$1
  local stop_triggered=$2

  _atomic_update_state "$session_id" \
    '.stop_triggered = $stop_triggered' \
    --argjson stop_triggered "$stop_triggered"
}

# set_paused() - Set paused flag in session state
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - paused value (true|false)
# Updates: {git-dir}/bumper-checkpoints/session-{sessionId} with new flag value
# Purpose: Temporarily suspend threshold enforcement while continuing to track changes
set_paused() {
  local session_id=$1
  local paused=$2

  _atomic_update_state "$session_id" \
    '.paused = $paused' \
    --argjson paused "$paused"
}

# update_incremental_state() - Update previous_tree and accumulated_score for incremental tracking
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - new_previous_tree (40-char git tree SHA to store as previous)
#   $3 - new_accumulated_score (integer points accumulated this session)
# Updates: {git-dir}/bumper-checkpoints/session-{sessionId} with new tracking values
update_incremental_state() {
  local session_id=$1
  local new_previous_tree=$2
  local new_accumulated_score=$3

  _atomic_update_state "$session_id" \
    '.previous_tree = $previous_tree | .accumulated_score = $accumulated_score' \
    --arg previous_tree "$new_previous_tree" \
    --argjson accumulated_score "$new_accumulated_score"
}

# _reset_baseline_internal() - Internal implementation for baseline reset
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - new_baseline_tree (40-char git tree SHA of current state)
#   $3 - new_baseline_branch (optional: branch name to update)
# Updates: Resets baseline_tree, previous_tree, accumulated_score to 0, stop_triggered to false
#          If branch provided, also updates baseline_branch
# WHY: Both after-commit and stale-detection resets do identical operations
_reset_baseline_internal() {
  local session_id=$1
  local new_baseline_tree=$2
  local new_baseline_branch=${3:-""}

  local jq_filter='.baseline_tree = $baseline_tree | .previous_tree = $previous_tree | .accumulated_score = 0 | .stop_triggered = false'

  if [[ -n "$new_baseline_branch" ]]; then
    jq_filter="$jq_filter | .baseline_branch = \$baseline_branch"
    _atomic_update_state "$session_id" \
      "$jq_filter" \
      --arg baseline_tree "$new_baseline_tree" \
      --arg previous_tree "$new_baseline_tree" \
      --arg baseline_branch "$new_baseline_branch"
  else
    _atomic_update_state "$session_id" \
      "$jq_filter" \
      --arg baseline_tree "$new_baseline_tree" \
      --arg previous_tree "$new_baseline_tree"
  fi
}

# reset_baseline_after_commit() - Reset baseline to current tree after git commit
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - new_baseline_tree (40-char git tree SHA of current state)
# Purpose: Auto-reset after successful git commit during enforced session
reset_baseline_after_commit() {
  _reset_baseline_internal "$@"
}

# reset_baseline_stale() - Reset baseline when detected as stale (branch switch)
# Args:
#   $1 - session_id (conversation UUID)
#   $2 - new_baseline_tree (40-char git tree SHA of current state)
#   $3 - new_baseline_branch (optional: update baseline_branch after switch)
# Purpose: Auto-reset when baseline tree is not reachable from current HEAD
reset_baseline_stale() {
  _reset_baseline_internal "$@"
}

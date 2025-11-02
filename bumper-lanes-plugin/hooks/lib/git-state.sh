#!/usr/bin/env bash
set -euo pipefail

# git-state.sh - Git tree capture and diff computation functions
# Purpose: Capture working tree state as git tree objects and compute diff statistics

# capture_tree() - Captures current working tree (including untracked files) as a git tree object
# Returns: 40-character tree SHA on stdout
# Uses temporary index to avoid modifying actual staging area
capture_tree() {
  local tmp_index
  tmp_index=$(mktemp)
  trap 'rm -f $tmp_index' EXIT

  export GIT_INDEX_FILE="$tmp_index"

  # Add tracked file changes
  git add -u . 2>/dev/null || true

  # Add untracked files (excluding standard ignores)
  git ls-files --others --exclude-standard 2>/dev/null | xargs -r git add 2>/dev/null || true

  # Write tree and capture SHA
  local tree_sha
  tree_sha=$(git write-tree 2>/dev/null)

  unset GIT_INDEX_FILE

  if [[ -z "$tree_sha" ]]; then
    echo "ERROR: Failed to capture tree" >&2
    return 1
  fi

  echo "$tree_sha"
  return 0
}

# compute_diff() - Computes diff statistics between two tree SHAs
# Args:
#   $1 - baseline_tree (40-char SHA)
#   $2 - current_tree (40-char SHA)
# Returns: git diff-tree --shortstat output on stdout
#   Format: "N files changed, X insertions(+), Y deletions(-)"
compute_diff() {
  local baseline_tree=$1
  local current_tree=$2

  if [[ -z "$baseline_tree" ]] || [[ -z "$current_tree" ]]; then
    echo "ERROR: compute_diff requires two tree SHAs" >&2
    return 1
  fi

  # Use git diff-tree to compare trees
  local diff_output
  diff_output=$(git diff-tree --shortstat "$baseline_tree" "$current_tree" 2>/dev/null)

  if [[ -z "$diff_output" ]]; then
    # No changes or error - return zero stats
    echo "0 files changed, 0 insertions(+), 0 deletions(-)"
    return 0
  fi

  echo "$diff_output"
  return 0
}

#!/usr/bin/env bash
set -euo pipefail

# git-state.sh - Git tree capture and diff computation functions
# Purpose: Capture working tree state as git tree objects and compute diff statistics

# capture_tree() - Captures current working tree (including untracked files) as a git tree object
# Returns: 40-character tree SHA on stdout
# Uses temporary index to avoid modifying actual staging area
capture_tree() {
  local tmp_index=$(mktemp)
  trap "rm -f $tmp_index" EXIT

  export GIT_INDEX_FILE="$tmp_index"

  # Initialize temp index with HEAD tree (or empty if no commits yet)
  if git rev-parse HEAD &>/dev/null; then
    git read-tree HEAD 2>/dev/null || true
  else
    git read-tree --empty 2>/dev/null || true
  fi

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

# get_current_branch() - Gets current branch name
# Returns: branch name on stdout, or empty if detached HEAD
get_current_branch() {
  git rev-parse --abbrev-ref HEAD 2>/dev/null || echo ""
}

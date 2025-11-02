#!/usr/bin/env bash
set -euo pipefail

# threshold.sh - Threshold calculation and diff statistics parsing
# Purpose: Calculate threshold from git diff output using simple line-count metric

# calculate_threshold() - Parses git diff-tree output and returns total lines changed
# Args:
#   $1 - diff_output (git diff-tree --shortstat output)
# Returns: Total lines changed (insertions + deletions) on stdout
calculate_threshold() {
  local diff_output=$1

  # Parse git diff-tree --shortstat output
  # Format: "N files changed, X insertions(+), Y deletions(-)"

  local insertions=0
  local deletions=0

  if [[ "$diff_output" =~ ([0-9]+)\ insertion ]]; then
    insertions=${BASH_REMATCH[1]}
  fi

  if [[ "$diff_output" =~ ([0-9]+)\ deletion ]]; then
    deletions=${BASH_REMATCH[1]}
  fi

  # Simple line count metric: additions + deletions
  local total=$((insertions + deletions))

  echo "$total"
  return 0
}

# parse_diff_stats() - Parses diff output into structured JSON for reporting
# Args:
#   $1 - diff_output (git diff-tree --shortstat output)
# Returns: JSON object with files_changed, lines_added, lines_deleted, total_lines_changed
parse_diff_stats() {
  local diff_output=$1

  local files_changed=0
  local insertions=0
  local deletions=0

  if [[ "$diff_output" =~ ([0-9]+)\ file ]]; then
    files_changed=${BASH_REMATCH[1]}
  fi

  if [[ "$diff_output" =~ ([0-9]+)\ insertion ]]; then
    insertions=${BASH_REMATCH[1]}
  fi

  if [[ "$diff_output" =~ ([0-9]+)\ deletion ]]; then
    deletions=${BASH_REMATCH[1]}
  fi

  local total=$((insertions + deletions))

  # Return JSON for structured consumption
  echo "{\"files_changed\":$files_changed,\"lines_added\":$insertions,\"lines_deleted\":$deletions,\"total_lines_changed\":$total}"
  return 0
}

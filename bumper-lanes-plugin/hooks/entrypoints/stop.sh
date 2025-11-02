#!/usr/bin/env bash
set -euo pipefail

# stop.sh - Stop hook for threshold enforcement
# Purpose: Check diff threshold when agent stops, block if exceeded

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"
source "$SCRIPT_DIR/../lib/threshold.sh"

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id')

# Hook is already executed in project directory (cwd field in JSON)

# Load session state
if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
  # No baseline - allow stop (fail open)
  echo "null"
  exit 0
fi

baseline_tree=$(echo "$session_state" | jq -r '.baseline_tree')
threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')

# Capture current working tree
current_tree=$(capture_tree)
if [[ -z "$current_tree" ]]; then
  echo "ERROR: Failed to capture current tree" >&2
  echo "null"
  exit 0 # Fail open
fi

# Compute diff statistics
diff_output=$(compute_diff "$baseline_tree" "$current_tree")

# Calculate threshold and check limit
total_lines=$(calculate_threshold "$diff_output")

if [[ $total_lines -le $threshold_limit ]]; then
  # Under threshold - allow stop
  echo "null"
  exit 0
fi

# Build block response
# Parse diff stats for detailed reporting
diff_stats=$(parse_diff_stats "$diff_output")

files_changed=$(echo "$diff_stats" | jq -r '.files_changed')
lines_added=$(echo "$diff_stats" | jq -r '.lines_added')
lines_deleted=$(echo "$diff_stats" | jq -r '.lines_deleted')

threshold_pct=$(awk "BEGIN {printf \"%.1f\", ($total_lines / $threshold_limit) * 100}")

# Build reason message
reason="⚠ Diff threshold exceeded: $total_lines/$threshold_limit lines changed (${threshold_pct}%).

Changes:
  $files_changed files changed, $lines_added insertions(+), $lines_deleted deletions(-)

Review your changes and run /bumper-reset to continue."

# Output block decision
jq -n \
  --arg decision "block" \
  --arg reason "$reason" \
  --argjson diff_stats "$diff_stats" \
  --argjson threshold_limit "$threshold_limit" \
  --argjson threshold_percentage "$threshold_pct" \
  '{
    decision: $decision,
    reason: $reason,
    diff_stats: ($diff_stats + {threshold_limit: $threshold_limit, threshold_percentage: $threshold_percentage})
  }'

exit 0

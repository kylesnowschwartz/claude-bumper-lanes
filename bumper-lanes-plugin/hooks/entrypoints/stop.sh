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
stop_hook_active=$(echo "$input" | jq -r '.stop_hook_active // false')

# Hook is already executed in project directory (cwd field in JSON)

# If already blocked once, allow stop this time to prevent infinite loop
if [[ "$stop_hook_active" == "true" ]]; then
  echo "null"
  exit 0
fi

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

# Over threshold - set stop_triggered flag to activate PreToolUse blocking
set_stop_triggered "$session_id" true

# Build block response
# Parse diff stats for detailed reporting
diff_stats=$(parse_diff_stats "$diff_output")

files_changed=$(echo "$diff_stats" | jq -r '.files_changed')
lines_added=$(echo "$diff_stats" | jq -r '.lines_added')
lines_deleted=$(echo "$diff_stats" | jq -r '.lines_deleted')

threshold_pct=$(awk "BEGIN {printf \"%.1f\", ($total_lines / $threshold_limit) * 100}")

# Build reason message
reason="⚠ Diff threshold exceeded: $total_lines/$threshold_limit lines changed (${threshold_pct}%).

Changes since baseline:
  $files_changed files changed, $lines_added insertions(+), $lines_deleted deletions(-)

STOP HERE - Do not continue working. The user must review changes first.

Tell the user:
1. Review the changes using 'git diff' or 'git status'
2. Run the /bumper-reset command when satisfied with the changes
3. This will accept the current changes as the new baseline
4. A fresh diff budget of $threshold_limit lines will be restored
5. After reset, the user can give you more work or end the session

This workflow ensures incremental code review at predictable checkpoints."

# Output block decision to STDOUT (JSON API pattern with exit code 0)
jq -n \
  --arg decision "block" \
  --arg reason "$reason" \
  --argjson continue false \
  --arg stopReason "Bumper-Lanes: ⚠ Diff threshold exceeded ($total_lines/$threshold_limit lines changed)" \
  --argjson diff_stats "$diff_stats" \
  --argjson threshold_limit "$threshold_limit" \
  --argjson threshold_percentage "$threshold_pct" \
  '{
    continue: $continue,
    stopReason: $stopReason,
    suppressOutput: false,
    decision: $decision,
    reason: $reason,
    diff_stats: ($diff_stats + {threshold_limit: $threshold_limit, threshold_percentage: $threshold_percentage})
  }'

exit 0

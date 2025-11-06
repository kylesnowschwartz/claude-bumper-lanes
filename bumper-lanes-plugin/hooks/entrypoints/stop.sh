#!/usr/bin/env bash
set -euo pipefail

# stop.sh - Stop hook for threshold enforcement
# Purpose: Check diff threshold when agent stops, block if exceeded

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"
source "$SCRIPT_DIR/../lib/threshold-calculator.sh"

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
previous_tree=$(echo "$session_state" | jq -r '.previous_tree // .baseline_tree')
accumulated_score=$(echo "$session_state" | jq -r '.accumulated_score // 0')

# Capture current working tree
current_tree=$(capture_tree)
if [[ -z "$current_tree" ]]; then
  echo "ERROR: Failed to capture current tree" >&2
  echo "null"
  exit 0 # Fail open
fi

# Compute incremental threshold (previous → current + accumulated)
threshold_data=$(calculate_incremental_threshold "$previous_tree" "$current_tree" "$accumulated_score")
weighted_score=$(echo "$threshold_data" | jq -r '.accumulated_score')

if [[ $weighted_score -le $threshold_limit ]]; then
  # Under threshold - allow stop and update incremental state
  update_incremental_state "$session_id" "$current_tree" "$weighted_score"
  echo "null"
  exit 0
fi

# Over threshold - set stop_triggered flag to activate PreToolUse blocking
set_stop_triggered "$session_id" true
# Also update incremental state
update_incremental_state "$session_id" "$current_tree" "$weighted_score"

# Format breakdown for user message
# Note: threshold_data contains both weighted_score (delta) and accumulated_score (total)
# For user display, we need to show the accumulated total, not just this turn's delta
threshold_data_for_display=$(echo "$threshold_data" | jq '.weighted_score = .accumulated_score')
breakdown=$(format_threshold_breakdown "$threshold_data_for_display" "$threshold_limit")

# Build reason message
reason="

⚠️  Bumper lanes: Diff threshold exceeded

$breakdown

Ask the User: Would you like to conduct a structured, manual review?

This workflow ensures incremental code review at predictable checkpoints.

"

threshold_pct=$(awk "BEGIN {printf \"%.0f\", ($weighted_score / $threshold_limit) * 100}")

# Output block decision to STDOUT (JSON API pattern with exit code 0)
jq -n \
  --arg decision "block" \
  --arg reason "$reason" \
  --argjson continue true \
  --arg systemMessage "/bumper-reset after code review." \
  --argjson threshold_data "$threshold_data" \
  --argjson threshold_limit "$threshold_limit" \
  --argjson threshold_percentage "$threshold_pct" \
  '{
    continue: $continue,
    systemMessage: $systemMessage,
    suppressOutput: true,
    decision: $decision,
    reason: $reason,
    threshold_data: ($threshold_data + {threshold_limit: $threshold_limit, threshold_percentage: $threshold_percentage})
  }'

exit 0

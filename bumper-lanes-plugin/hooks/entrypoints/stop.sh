#!/usr/bin/env bash
set -euo pipefail

# stop.sh - Stop hook for threshold enforcement
# Purpose: Check diff threshold when agent stops, block if exceeded

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"
source "$SCRIPT_DIR/../lib/threshold.sh"
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

# Capture current working tree
current_tree=$(capture_tree)
if [[ -z "$current_tree" ]]; then
  echo "ERROR: Failed to capture current tree" >&2
  echo "null"
  exit 0 # Fail open
fi

# Compute weighted threshold using new calculator
threshold_data=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")
weighted_score=$(echo "$threshold_data" | jq -r '.weighted_score')

if [[ $weighted_score -le $threshold_limit ]]; then
  # Under threshold - allow stop
  echo "null"
  exit 0
fi

# Over threshold - set stop_triggered flag to activate PreToolUse blocking
set_stop_triggered "$session_id" true

# Format breakdown for user message
breakdown=$(format_threshold_breakdown "$threshold_data" "$threshold_limit")

# Build reason message
reason="⚠️  Bumper lanes: Diff threshold exceeded

$breakdown

STOP HERE - Do not continue working. The user must review changes first.

Tell the user:
1. Review the changes using 'git diff' or 'git status'
2. Run the /bumper-reset command when satisfied with the changes
3. This will accept the current changes as the new baseline
4. A fresh diff budget of $threshold_limit points will be restored
5. After reset, the user can give you more work or end the session

This workflow ensures incremental code review at predictable checkpoints.

Research note: GitLab recommends ~200 lines per merge request for optimal review effectiveness (70-90% defect detection)."

threshold_pct=$(awk "BEGIN {printf \"%.0f\", ($weighted_score / $threshold_limit) * 100}")

# Output block decision to STDOUT (JSON API pattern with exit code 0)
jq -n \
  --arg decision "block" \
  --arg reason "$reason" \
  --argjson continue false \
  --arg stopReason "Bumper-Lanes: ⚠️ Diff threshold exceeded ($weighted_score/$threshold_limit points, ${threshold_pct}%)" \
  --argjson threshold_data "$threshold_data" \
  --argjson threshold_limit "$threshold_limit" \
  --argjson threshold_percentage "$threshold_pct" \
  '{
    continue: $continue,
    stopReason: $stopReason,
    suppressOutput: false,
    decision: $decision,
    reason: $reason,
    threshold_data: ($threshold_data + {threshold_limit: $threshold_limit, threshold_percentage: $threshold_percentage})
  }'

exit 0

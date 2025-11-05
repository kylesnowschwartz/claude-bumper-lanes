#!/usr/bin/env bash
set -euo pipefail

# pre-tool-use.sh - PreToolUse hook for threshold enforcement
# Purpose: Block file modification tools when diff threshold is exceeded

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"
source "$SCRIPT_DIR/../lib/threshold.sh"

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id')
tool_name=$(echo "$input" | jq -r '.tool_name')
hook_event_name=$(echo "$input" | jq -r '.hook_event_name')

# Validate hook event (defensive check)
if [[ "$hook_event_name" != "PreToolUse" ]]; then
  echo "null"
  exit 0
fi

# Check if tool is a modification tool (early return optimization)
case "$tool_name" in
  Write|Edit)
    # This is a file modification tool - proceed with threshold check
    ;;
  *)
    # Not a file modification tool - allow it immediately
    echo "null"
    exit 0
    ;;
esac

# Load session state
if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
  # No session state - fail open (allow tool)
  echo "null"
  exit 0
fi

# Extract state
baseline_tree=$(echo "$session_state" | jq -r '.baseline_tree')
threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')
stop_triggered=$(echo "$session_state" | jq -r '.stop_triggered // false')

# Check if Stop hook has been triggered
# PreToolUse only blocks AFTER Stop hook has fired once
if [[ "$stop_triggered" != "true" ]]; then
  # Stop hasn't triggered yet - allow tool to proceed
  echo "null"
  exit 0
fi

# Stop has triggered - now enforce blocking on Write/Edit tools
# Capture current working tree
if ! current_tree=$(capture_tree 2>/dev/null); then
  # Failed to capture tree - fail open
  echo "null"
  exit 0
fi

# Compute diff statistics
diff_output=$(compute_diff "$baseline_tree" "$current_tree")
total_lines=$(calculate_threshold "$diff_output")

# Check threshold
if [[ $total_lines -le $threshold_limit ]]; then
  # Under threshold - allow tool
  echo "null"
  exit 0
fi

# Over threshold - deny tool call
# Parse diff stats for detailed message
diff_stats=$(parse_diff_stats "$diff_output")
files_changed=$(echo "$diff_stats" | jq -r '.files_changed')
lines_added=$(echo "$diff_stats" | jq -r '.lines_added')
lines_deleted=$(echo "$diff_stats" | jq -r '.lines_deleted')

threshold_pct=$(awk "BEGIN {printf \"%.1f\", ($total_lines / $threshold_limit) * 100}")

# Build denial reason
reason="⚠ Diff threshold exceeded: $total_lines/$threshold_limit lines changed (${threshold_pct}%).

Cannot modify files while over threshold.

Current changes:
  $files_changed files, $lines_added insertions(+), $lines_deleted deletions(-)

Review your changes and run /bumper-reset to continue."

# Output denial decision using modern JSON format
jq -n \
  --arg reason "$reason" \
  '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $reason
    }
  }'

exit 0

#!/usr/bin/env bash
set -euo pipefail

# pre-tool-use.sh - PreToolUse hook for threshold enforcement
# Purpose: Block file modification tools when diff threshold is exceeded

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"
source "$SCRIPT_DIR/../lib/threshold.sh"
source "$SCRIPT_DIR/../lib/threshold-calculator.sh"

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
Write | Edit)
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

# Compute weighted threshold using new calculator
threshold_data=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")
weighted_score=$(echo "$threshold_data" | jq -r '.weighted_score')

# Check threshold
if [[ $weighted_score -le $threshold_limit ]]; then
  # Under threshold - allow tool
  echo "null"
  exit 0
fi

# Over threshold - deny tool call
# Format breakdown for user message
breakdown=$(format_threshold_breakdown "$threshold_data" "$threshold_limit")

# Build denial reason
reason="🚫 Bumper lanes: Diff threshold exceeded

$breakdown

Cannot modify files while over threshold.

Review your changes and run /bumper-reset to continue.

Tip: GitLab recommends ~200 lines per merge request for optimal review."

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

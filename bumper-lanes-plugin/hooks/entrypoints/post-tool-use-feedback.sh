#!/usr/bin/env bash
set -euo pipefail

# post-tool-use-feedback.sh - PostToolUse hook for fuel gauge feedback
# Purpose: Provide threshold feedback to Claude after Write/Edit operations
# Mechanism: stderr + exit 2 → reaches Claude (per docs)

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"
source "$SCRIPT_DIR/../lib/threshold-calculator.sh"

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id')
tool_name=$(echo "$input" | jq -r '.tool_name')
hook_event_name=$(echo "$input" | jq -r '.hook_event_name')

# Validate hook event (defensive check)
if [[ "$hook_event_name" != "PostToolUse" ]]; then
  exit 0
fi

# Only process Write/Edit tools
case "$tool_name" in
Write | Edit)
  # Proceed with threshold check
  ;;
*)
  exit 0
  ;;
esac

# Load session state
if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
  # No session state - fail open
  exit 0
fi

# Extract state
baseline_tree=$(echo "$session_state" | jq -r '.baseline_tree')
threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')
previous_tree=$(echo "$session_state" | jq -r '.previous_tree // .baseline_tree')
accumulated_score=$(echo "$session_state" | jq -r '.accumulated_score // 0')
paused=$(echo "$session_state" | jq -r '.paused // false')

# If paused, exit silently
if [[ "$paused" == "true" ]]; then
  exit 0
fi

# Capture current working tree
if ! current_tree=$(capture_tree 2>/dev/null); then
  exit 0
fi

# Calculate threshold
threshold_data=$(calculate_incremental_threshold "$previous_tree" "$current_tree" "$accumulated_score")
score=$(echo "$threshold_data" | jq -r '.accumulated_score')

# Update incremental state for next check
update_incremental_state "$session_id" "$current_tree" "$score"

# Calculate percentage
pct=$((score * 100 / threshold_limit))

# Output fuel gauge to stderr based on threshold tier
# Exit 2 ensures stderr reaches Claude (per docs)
if [[ $pct -ge 90 ]]; then
  echo "CRITICAL: Review budget near critical ($pct%). $score/$threshold_limit pts. STOP accepting work. Inform user checkpoint needed NOW." >&2
  exit 2
elif [[ $pct -ge 75 ]]; then
  echo "WARNING: Review budget at $pct% ($score/$threshold_limit pts). Complete current work, then ask user about checkpoint." >&2
  exit 2
elif [[ $pct -ge 50 ]]; then
  echo "NOTICE: $pct% budget used ($score/$threshold_limit pts). Wrap up current task soon." >&2
  exit 2
fi

# Under 50% - silent
exit 0

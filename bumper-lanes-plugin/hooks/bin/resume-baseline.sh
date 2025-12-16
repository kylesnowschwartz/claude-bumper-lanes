#!/usr/bin/env bash
set -euo pipefail

# resume-baseline.sh - Resume threshold enforcement (NOT a hook)
# Purpose: Re-enable Write/Edit blocking after a pause

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"

# Read command-line argument (sessionId passed from command)
session_id=${1:-}

if [[ -z "$session_id" ]]; then
  echo "Warning: No session ID provided"
  exit 1
fi

# Load session state
if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
  echo "Warning: No active session found. Cannot resume."
  exit 0
fi

# Check if already unpaused
paused=$(echo "$session_state" | jq -r '.paused // false')
if [[ "$paused" != "true" ]]; then
  echo "Bumper lanes not paused. Enforcement is already active."
  exit 0
fi

# Get current score for status message
accumulated_score=$(echo "$session_state" | jq -r '.accumulated_score // 0')
threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')
stop_triggered=$(echo "$session_state" | jq -r '.stop_triggered // false')

# Clear paused flag
set_paused "$session_id" false

# Calculate percentage for status
pct=$((accumulated_score * 100 / threshold_limit))

# Check if over threshold and warn
if [[ $accumulated_score -gt $threshold_limit ]]; then
  cat <<EOF
Warning: Bumper lanes resumed — OVER THRESHOLD ($accumulated_score/$threshold_limit pts, ${pct}%)

Write/Edit operations will be blocked until /bumper-reset.
Review your changes before resetting.
EOF
elif [[ $pct -ge 75 ]]; then
  cat <<EOF
Bumper lanes: Enforcement resumed ($accumulated_score/$threshold_limit pts, ${pct}%)

Approaching threshold. Consider committing working state soon.
EOF
else
  cat <<EOF
Bumper lanes: Enforcement resumed ($accumulated_score/$threshold_limit pts)

Threshold checks active.
EOF
fi

exit 0

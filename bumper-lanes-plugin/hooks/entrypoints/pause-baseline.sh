#!/usr/bin/env bash
set -euo pipefail

# pause-baseline.sh - Pause threshold enforcement (NOT a hook)
# Purpose: Temporarily suspend Write/Edit blocking while continuing to track changes

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
  echo "Warning: No active session found. Cannot pause."
  exit 0
fi

# Check if already paused
paused=$(echo "$session_state" | jq -r '.paused // false')
if [[ "$paused" == "true" ]]; then
  echo "Bumper lanes already paused. Use /bumper-resume to re-enable enforcement."
  exit 0
fi

# Get current score for status message
accumulated_score=$(echo "$session_state" | jq -r '.accumulated_score // 0')
threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')

# Set paused flag
set_paused "$session_id" true

cat <<EOF
Bumper lanes: Enforcement paused ($accumulated_score/$threshold_limit pts)

Edit/Write operations will proceed without threshold checks.
Score tracking continues in the background.

Use /bumper-resume to re-enable enforcement.
EOF

exit 0

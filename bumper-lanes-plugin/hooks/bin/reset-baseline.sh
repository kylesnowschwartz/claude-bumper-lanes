#!/usr/bin/env bash
set -euo pipefail

# reset-baseline.sh - Reset baseline script (NOT a hook)
# Purpose: Reset baseline tree to current working tree state, update session state

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"

# Read command-line argument (sessionId passed from command)
session_id=${1:-}

if [[ -z "$session_id" ]]; then
  echo "⚠ Bumper Lanes: Error - No session ID provided"
  exit 1
fi

# Load session state
if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
  # No active session - print error message
  echo "⚠ Bumper Lanes: No active session found. Baseline reset skipped."
  exit 0
fi

old_baseline=$(echo "$session_state" | jq -r '.baseline_tree')
threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')
created_at=$(echo "$session_state" | jq -r '.created_at')

# Capture current working tree state
current_tree=$(capture_tree)
if [[ -z "$current_tree" ]]; then
  echo "⚠ Bumper Lanes: Failed to reset baseline. Please try again."
  exit 1
fi

# Get diff summary for reporting (inline, no separate function needed)
diff_output=$(git diff-tree --shortstat "$old_baseline" "$current_tree" 2>/dev/null || echo "0 files changed")

# Parse diff stats inline (git diff-tree --shortstat format)
# Format: "N files changed, X insertions(+), Y deletions(-)"
files_changed=0
lines_added=0
lines_deleted=0

if [[ "$diff_output" =~ ([0-9]+)\ file ]]; then
  files_changed=${BASH_REMATCH[1]}
fi
if [[ "$diff_output" =~ ([0-9]+)\ insertion ]]; then
  lines_added=${BASH_REMATCH[1]}
fi
if [[ "$diff_output" =~ ([0-9]+)\ deletion ]]; then
  lines_deleted=${BASH_REMATCH[1]}
fi

total_lines=$((lines_added + lines_deleted))

# Update session state with new baseline and clear incremental tracking
new_baseline="$current_tree"
write_session_state "$session_id" "$new_baseline"
set_stop_triggered "$session_id" false
# Reset incremental tracking: previous_tree = baseline, accumulated_score = 0
update_incremental_state "$session_id" "$new_baseline" 0

# Build confirmation message
# Format timestamps for display
old_timestamp=$(date -r "$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$created_at" +%s)" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "$created_at")
new_timestamp=$(date "+%Y-%m-%d %H:%M:%S")

# Truncate SHAs for display
old_baseline_short="${old_baseline:0:7}"
new_baseline_short="${new_baseline:0:7}"

# Build multi-line confirmation message
cat <<EOF
✓ Baseline reset complete.

Previous baseline: $old_baseline_short (captured $old_timestamp)
New baseline: $new_baseline_short (captured $new_timestamp)

Changes accepted: $files_changed files, $lines_added insertions(+), $lines_deleted deletions(-) [$total_lines lines total]

You now have a fresh diff budget of $threshold_limit points. Pick up where we left off?
EOF

exit 0

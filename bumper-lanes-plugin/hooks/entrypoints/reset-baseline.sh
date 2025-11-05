#!/usr/bin/env bash
set -euo pipefail

# reset-baseline.sh - Reset baseline script (NOT a hook)
# Purpose: Reset baseline tree to current working tree state, update session state

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"
source "$SCRIPT_DIR/../lib/threshold.sh"

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

# Compute final diff stats (for reporting accepted changes)
current_tree=$(capture_tree)
if [[ -z "$current_tree" ]]; then
  echo "⚠ Bumper Lanes: Failed to reset baseline. Please try again."
  exit 1
fi

diff_output=$(compute_diff "$old_baseline" "$current_tree")
diff_stats=$(parse_diff_stats "$diff_output")
total_lines=$(echo "$diff_stats" | jq -r '.total_lines_changed')

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

# Extract stats for message
files_changed=$(echo "$diff_stats" | jq -r '.files_changed')
lines_added=$(echo "$diff_stats" | jq -r '.lines_added')
lines_deleted=$(echo "$diff_stats" | jq -r '.lines_deleted')

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

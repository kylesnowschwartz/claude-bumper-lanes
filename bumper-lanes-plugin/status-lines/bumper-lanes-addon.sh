#!/bin/bash
# bumper-lanes-addon.sh - Status line addon for bumper-lanes
#
# Usage: Set as statusLine.command in ~/.claude/settings.json:
#   { "statusLine": { "command": "/path/to/bumper-lanes-addon.sh" } }
#
# Outputs (multi-line status line):
#   Line 1: Indicator - "active (125/400 - 31%)" or "tripped (412/400 - 103%)"
#   Line 2+: Diff tree visualization showing changed files
#
# ─────────────────────────────────────────────────────────────────────────────
# For custom status lines, cherry-pick these widgets:
#
#   # At the start of your script:
#   input=$(cat)
#   BUMPER_LANES="/path/to/bumper-lanes"  # or run: /bumper-setup-statusline
#
#   # Append to your output:
#   indicator=$(echo "$input" | "$BUMPER_LANES" status --widget=indicator)
#   diff_tree=$(echo "$input" | "$BUMPER_LANES" status --widget=diff-tree)
#   [[ -n "$indicator" ]] && echo "$indicator"
#   [[ -n "$diff_tree" ]] && echo "$diff_tree"
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

# Resolve binary path relative to this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUMPER_LANES="$SCRIPT_DIR/../bin/bumper-lanes"

# Verify binary exists
if [[ ! -x "$BUMPER_LANES" ]]; then
  echo "bumper-lanes: binary not found" >&2
  exit 0 # Fail silently - don't break status line
fi

# Read JSON from stdin
input=$(cat)

# Output indicator (single line)
indicator=$(echo "$input" | "$BUMPER_LANES" status --widget=indicator 2>/dev/null || true)
[[ -n "$indicator" ]] && echo "$indicator"

# Output diff tree (multi-line)
diff_tree=$(echo "$input" | "$BUMPER_LANES" status --widget=diff-tree 2>/dev/null || true)
[[ -n "$diff_tree" ]] && echo "$diff_tree"

exit 0

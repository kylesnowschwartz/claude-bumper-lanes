#!/usr/bin/env bash
set -euo pipefail

# set-view-mode.sh - Set diff visualization mode for status line
# Usage: set-view-mode.sh <session_id> <mode>
# Modes: tree, collapsed, sparkline

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/state-manager.sh"

session_id=${1:-}
view_mode=${2:-}

if [[ -z "$session_id" ]]; then
  echo "ERROR: session_id required" >&2
  exit 1
fi

if [[ -z "$view_mode" ]]; then
  # No mode specified - show current mode
  current=$(get_view_mode "$session_id")
  echo "Current view mode: $current"
  echo "Available modes: tree, collapsed, sparkline, smart, hier, stacked"
  exit 0
fi

# Set the new mode
if set_view_mode "$session_id" "$view_mode"; then
  echo "View mode set to: $view_mode"
else
  exit 1
fi

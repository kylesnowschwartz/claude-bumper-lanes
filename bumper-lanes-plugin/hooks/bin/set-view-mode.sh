#!/usr/bin/env bash
set -euo pipefail

# set-view-mode.sh - Set diff visualization mode for status line
# Usage: set-view-mode.sh <session_id> <mode>
# Sets both session state (immediate) and personal config (persistent)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/state-manager.sh"

session_id=${1:-}
view_mode=${2:-}

if [[ -z "$session_id" ]]; then
  echo "ERROR: session_id required" >&2
  exit 1
fi

# Get valid modes from binary (with fallback)
valid_modes=$(git-diff-tree-go --list-modes 2>/dev/null) || valid_modes="tree collapsed smart topn icicle brackets"

if [[ -z "$view_mode" ]]; then
  # No mode specified - show current mode and config
  current=$(get_view_mode "$session_id")
  default=$(get_default_view_mode)
  echo "Current view mode: $current"
  echo "Default (from config): $default"
  echo "Available modes: $valid_modes"
  exit 0
fi

# Validate mode
if ! echo " $valid_modes " | grep -q " $view_mode "; then
  echo "ERROR: Invalid view mode '$view_mode'" >&2
  echo "Valid modes: $valid_modes" >&2
  exit 1
fi

# Set session state (immediate effect)
if ! set_view_mode "$session_id" "$view_mode"; then
  echo "ERROR: Failed to set session view mode" >&2
  exit 1
fi

# Persist to personal config (.git/bumper-config.json)
git_dir=$(git rev-parse --absolute-git-dir 2>/dev/null) || {
  echo "View mode set to: $view_mode (session only - not in git repo)"
  exit 0
}

personal_config="$git_dir/bumper-config.json"

if [[ -f "$personal_config" ]]; then
  temp_file=$(mktemp)
  jq --arg mode "$view_mode" '.default_view_mode = $mode' "$personal_config" >"$temp_file"
  mv "$temp_file" "$personal_config"
else
  echo "{\"default_view_mode\": \"$view_mode\"}" >"$personal_config"
fi

echo "View mode set to: $view_mode"
echo "Saved to: $personal_config"

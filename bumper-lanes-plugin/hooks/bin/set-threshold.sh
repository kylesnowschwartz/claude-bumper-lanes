#!/usr/bin/env bash
set -euo pipefail

# set-threshold.sh - Show or set threshold configuration
# Usage: set-threshold.sh <session_id> [show|set <threshold>|personal <threshold>]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/state-manager.sh"

session_id="${1:-}"
action="${2:-show}"
value="${3:-}"

if [[ -z "$session_id" ]]; then
  echo "ERROR: session_id required" >&2
  exit 1
fi

git_dir=$(git rev-parse --absolute-git-dir 2>/dev/null) || {
  echo "ERROR: Not in a git repository" >&2
  exit 1
}
repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "ERROR: Cannot find repo root" >&2
  exit 1
}

personal_config="$git_dir/bumper-config.json"
repo_config="$repo_root/.bumper-lanes.json"

show_config() {
  echo "Bumper Lanes Configuration"
  echo "=========================="
  echo ""

  # Show effective threshold
  local effective
  effective=$(get_threshold_limit)
  echo "Effective threshold: $effective points"
  echo ""

  # Show config file status
  echo "Config files (priority order):"
  echo ""

  # Personal config
  if [[ -f "$personal_config" ]]; then
    local personal_val
    personal_val=$(jq -r '.threshold // "not set"' "$personal_config" 2>/dev/null || echo "invalid")
    echo "  1. Personal: $personal_config"
    echo "     threshold: $personal_val"
  else
    echo "  1. Personal: (not created)"
    echo "     Location: $personal_config"
  fi
  echo ""

  # Repo config
  if [[ -f "$repo_config" ]]; then
    local repo_val
    repo_val=$(jq -r '.threshold // "not set"' "$repo_config" 2>/dev/null || echo "invalid")
    echo "  2. Repo: $repo_config"
    echo "     threshold: $repo_val"
  else
    echo "  2. Repo: (not created)"
    echo "     Location: $repo_config"
  fi
  echo ""

  echo "  3. Default: $DEFAULT_THRESHOLD points"
  echo ""
  echo "Use '/bumper-config set <value>' to set repo threshold"
  echo "Use '/bumper-config personal <value>' for personal override"
}

set_repo_config() {
  local threshold="$1"

  if ! [[ "$threshold" =~ ^[0-9]+$ ]]; then
    echo "ERROR: Threshold must be a positive integer" >&2
    exit 1
  fi

  if [[ "$threshold" -lt 50 ]]; then
    echo "ERROR: Threshold too low (minimum 50)" >&2
    exit 1
  fi

  if [[ "$threshold" -gt 2000 ]]; then
    echo "ERROR: Threshold too high (maximum 2000)" >&2
    exit 1
  fi

  # Create or update repo config
  if [[ -f "$repo_config" ]]; then
    local temp_file
    temp_file=$(mktemp)
    jq --argjson threshold "$threshold" '.threshold = $threshold' "$repo_config" >"$temp_file"
    mv "$temp_file" "$repo_config"
  else
    echo "{\"threshold\": $threshold}" >"$repo_config"
  fi

  echo "Set repo threshold to $threshold points"
  echo "File: $repo_config"
  echo ""
  echo "Note: Run /bumper-reset to apply to current session"
}

set_personal_config() {
  local threshold="$1"

  if ! [[ "$threshold" =~ ^[0-9]+$ ]]; then
    echo "ERROR: Threshold must be a positive integer" >&2
    exit 1
  fi

  if [[ "$threshold" -lt 50 ]]; then
    echo "ERROR: Threshold too low (minimum 50)" >&2
    exit 1
  fi

  if [[ "$threshold" -gt 2000 ]]; then
    echo "ERROR: Threshold too high (maximum 2000)" >&2
    exit 1
  fi

  # Create or update personal config
  if [[ -f "$personal_config" ]]; then
    local temp_file
    temp_file=$(mktemp)
    jq --argjson threshold "$threshold" '.threshold = $threshold' "$personal_config" >"$temp_file"
    mv "$temp_file" "$personal_config"
  else
    echo "{\"threshold\": $threshold}" >"$personal_config"
  fi

  echo "Set personal threshold to $threshold points"
  echo "File: $personal_config"
  echo "(This file is in .git/ and won't be tracked)"
  echo ""
  echo "Note: Run /bumper-reset to apply to current session"
}

case "$action" in
show)
  show_config
  ;;
set)
  if [[ -z "$value" ]]; then
    echo "ERROR: Threshold value required" >&2
    echo "Usage: /bumper-config set <threshold>" >&2
    exit 1
  fi
  set_repo_config "$value"
  ;;
personal)
  if [[ -z "$value" ]]; then
    echo "ERROR: Threshold value required" >&2
    echo "Usage: /bumper-config personal <threshold>" >&2
    exit 1
  fi
  set_personal_config "$value"
  ;;
*)
  echo "ERROR: Unknown action '$action'" >&2
  echo "Usage: /bumper-config [show|set <threshold>|personal <threshold>]" >&2
  exit 1
  ;;
esac

exit 0

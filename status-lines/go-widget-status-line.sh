#!/bin/bash
# Minimal status line using Go bumper-lanes widget
# This demonstrates how to integrate the widget into any status line

input=$(cat)
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bumper_bin="$script_dir/../bumper-lanes-plugin/bin/bumper-lanes"

# Color codes (use $'...' for proper escape interpretation)
c_magenta=$'\e[95m'
c_blue=$'\e[94m'
c_yellow=$'\e[33m'
c_cost=$'\e[35m'
c_reset=$'\e[0m'

# Extract basics from JSON
model=$(echo "$input" | jq -r '.model.display_name // "?"')
dir=$(echo "$input" | jq -r '.workspace.current_dir // "?"')
dir_name="${dir##*/}"

# Git branch with clean/dirty indicator (starship-style)
branch=$(git branch --show-current 2>/dev/null)
if [[ -n "$branch" ]]; then
  if git diff --quiet HEAD 2>/dev/null; then
    git_status="${c_blue}${branch}${c_reset}"
  else
    git_status="${c_blue}${branch}${c_reset} ${c_yellow}*${c_reset}"
  fi
fi

# Cost
cost=$(echo "$input" | jq -r '.cost.total_cost_usd // 0')
cost_fmt=$(printf "\$%.2f" "$cost")

# Build main line
output="${c_magenta}[${model}]${c_reset} ${dir_name}"
[[ -n "$git_status" ]] && output+=" | $git_status"
output+=" | ${c_cost}${cost_fmt}${c_reset}"

# Get bumper-lanes widget from Go binary
if [[ -x "$bumper_bin" ]]; then
  widget_output=$(echo "$input" | "$bumper_bin" status 2>/dev/null)
  if [[ -n "$widget_output" ]]; then
    # First line is status, rest is diff tree
    first_line=$(echo "$widget_output" | head -1)
    rest=$(echo "$widget_output" | tail -n +2)

    [[ -n "$first_line" ]] && output+=" | $first_line"
  fi
fi

printf '%s\n' "$output"

# Output diff tree lines with non-breaking space preservation
if [[ -n "$rest" ]]; then
  while IFS= read -r line; do
    [[ -n "$line" ]] && printf '\e[0m%s\n' "${line// /$'\xc2\xa0'}"
  done <<<"$rest"
fi

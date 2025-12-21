#!/bin/bash
# Read JSON input once
input=$(cat)

# Helper functions for common extractions
get_model_name() { echo "$input" | jq -r '.model.display_name'; }
get_current_dir() { echo "$input" | jq -r '.workspace.current_dir'; }
get_cost() { echo "$input" | jq -r '.cost.total_cost_usd'; }
get_transcript_path() { echo "$input" | jq -r '.transcript_path // ""'; }
get_session_id() { echo "$input" | jq -r '.session_id'; }

# Color helper functions
color_model() { printf "\e[95m%s\e[0m" "$1"; }          # magenta
color_branch() { printf "\e[94m%s\e[0m" "$1"; }         # blue
color_changes() { printf "\e[32m%s\e[0m" "$1"; }        # green
color_tokens() { printf "\e[33m%s\e[0m" "$1"; }         # yellow
color_waiting() { printf "\e[36m%s\e[0m" "$1"; }        # cyan
color_cost() { printf "\e[35m%s\e[0m" "$1"; }           # magenta
color_bumper_active() { printf "\e[32m%s\e[0m" "$1"; }  # green
color_bumper_tripped() { printf "\e[31m%s\e[0m" "$1"; } # red
color_session_id() { printf "\e[90m%s\e[0m" "$1"; }     # dim gray

# Git helper functions
get_git_branch() {
  local branch=$(git branch --show-current 2>/dev/null)
  [[ -n "$branch" ]] && echo "$branch"
}
get_git_changes() {
  # Fast git diff stats using single call - gets all changes (staged + unstaged)
  # Returns formatted string like "+55 -34" with colors, or empty string if no changes/errors
  local changes insertions=0 deletions=0
  changes=$(git diff HEAD --shortstat 2>/dev/null) || return

  [[ -z "$changes" ]] && return

  # Extract insertions and deletions in single operations
  [[ "$changes" =~ ([0-9]+)\ insertions? ]] && insertions=${BASH_REMATCH[1]}
  [[ "$changes" =~ ([0-9]+)\ deletions? ]] && deletions=${BASH_REMATCH[1]}

  # Only output if there are actual changes
  # e.g. +5 -1 with green + and red -
  ((insertions > 0 || deletions > 0)) && printf "\e[32m+%d\e[0m \e[31m-%d\e[0m" "$insertions" "$deletions"
}

get_total_tokens() {
  local transcript=$(get_transcript_path)
  [[ ! -f "$transcript" ]] && echo "0" && return

  jq -s -r '
  [
  .[] |
    select(.isSidechain == false) |
    select(.usage // .message.usage) |
    select((.message.model // "" | ascii_downcase) | (. != "<synthetic>" and (contains("synthetic") | not))) |
    select(.isApiErrorMessage != true) |
    select(
      ([.message.content[]? | select(.type == "text")] | length == 0) or
      ([.message.content[]? | select(.type == "text" and (.text | test("no\\\\s+response\\\\s+requested"; "i")))] | length == 0)
      ) |
        {
          timestamp,
          total: ((.usage // .message.usage) // {} |
            (.input_tokens // 0) +
          (.output_tokens // 0) +
          (.cache_read_input_tokens // 0) +
          (.cache_creation_input_tokens // 0)
        )
      } |
        select(.total > 0)
      ] |
        if length > 0 then
          max_by(.timestamp).total
        else
          0
        end
        ' "$transcript" 2>/dev/null || echo "0"
}

get_bumper_lanes_status() {
  local session_id=$(echo "$input" | jq -r '.session_id')
  local workspace_dir=$(echo "$input" | jq -r '.workspace.current_dir // ""')
  [[ -z "$workspace_dir" ]] && return

  local git_dir
  git_dir=$(git -C "$workspace_dir" rev-parse --absolute-git-dir 2>/dev/null) || return
  local checkpoint_file="$git_dir/bumper-checkpoints/session-$session_id"

  # No session file = bumper lanes not active
  [[ ! -f "$checkpoint_file" ]] && return

  # Read state flags
  local stop_triggered=$(jq -r '.stop_triggered // false' "$checkpoint_file" 2>/dev/null)
  local paused=$(jq -r '.paused // false' "$checkpoint_file" 2>/dev/null)
  local accumulated_score=$(jq -r '.accumulated_score // 0' "$checkpoint_file" 2>/dev/null)
  local threshold_limit=$(jq -r '.threshold_limit // 400' "$checkpoint_file" 2>/dev/null)

  # Calculate percentage (avoid division by zero)
  local percentage=0
  if [[ "$threshold_limit" -gt 0 ]]; then
    percentage=$(awk "BEGIN {printf \"%.0f\", ($accumulated_score / $threshold_limit) * 100}")
  fi

  # Return format: "state score/limit percentage"
  # States: active, tripped, paused
  if [[ "$paused" == "true" ]]; then
    echo "paused $accumulated_score $threshold_limit $percentage"
  elif [[ "$stop_triggered" == "true" ]]; then
    echo "tripped $accumulated_score $threshold_limit $percentage"
  else
    echo "active $accumulated_score $threshold_limit $percentage"
  fi
}

# Get hierarchical diff tree (if git-diff-tree is available and there are changes)
get_diff_tree() {
  local workspace_dir=$(echo "$input" | jq -r '.workspace.current_dir // ""')
  [[ -z "$workspace_dir" ]] && return

  # Find git-diff-tree-go (Go version) relative to this script
  local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  local diff_tree_bin="$script_dir/../bumper-lanes-plugin/bin/git-diff-tree-go"

  # Fallback to bash version if Go binary not found
  [[ ! -x "$diff_tree_bin" ]] && diff_tree_bin="$script_dir/../bumper-lanes-plugin/bin/git-diff-tree"
  [[ ! -x "$diff_tree_bin" ]] && return

  # Run from workspace directory
  local tree_output
  tree_output=$(cd "$workspace_dir" && "$diff_tree_bin" 2>/dev/null) || return

  # Only return if there's actual content (not "No changes")
  [[ "$tree_output" == "No changes" ]] && return

  echo "$tree_output"
}

# Use the helpers
MODEL=$(get_model_name)
DIR=$(get_current_dir)
TOKENS=$(get_total_tokens)
BRANCH=$(get_git_branch)
CHANGES=$(get_git_changes)
COST=$(get_cost)
BUMPER_STATUS=$(get_bumper_lanes_status)
SESSION_ID=$(get_session_id)
DIFF_TREE=$(get_diff_tree)

# Build output string with color helpers
output="[$(color_model "$MODEL")] 📁 ${DIR##*/}"
[[ -n "$BRANCH" ]] && output+=" | $(color_branch "$BRANCH")"
[[ -n "$CHANGES" ]] && output+=" $CHANGES"
if [[ "$TOKENS" != "0" && -n "$TOKENS" ]]; then
  output+=" | $(color_tokens "$TOKENS")"
else
  output+=" | $(color_waiting "...waiting")"
fi

if [[ "$COST" != "null" && -n "$COST" ]]; then
  rounded_cost=$(printf "%.2f" "$COST" 2>/dev/null || echo "0.00")
  output+=" | $(color_cost "\$$rounded_cost")"
else
  output+=" | $(color_cost "\$0.00")"
fi

# Add bumper lanes status if active
if [[ -n "$BUMPER_STATUS" ]]; then
  # Parse the status string: "state score limit percentage"
  read -r state score limit percentage <<<"$BUMPER_STATUS"

  if [[ "$state" == "paused" ]]; then
    # Paused: show action hint instead of diff stats
    status_text="Paused: run /bumper-resume"
    output+=" | $(color_tokens "$status_text")"
  elif [[ "$state" == "active" ]]; then
    status_text="bumper-lanes $state ($score/$limit · $percentage%)"
    output+=" | $(color_bumper_active "$status_text")"
  elif [[ "$state" == "tripped" ]]; then
    status_text="bumper-lanes $state ($score/$limit · $percentage%)"
    output+=" | $(color_bumper_tripped "$status_text")"
  fi
fi

# Add session ID (always show, dimmed)
if [[ -n "$SESSION_ID" && "$SESSION_ID" != "null" ]]; then
  # Show first 8 chars of session ID for brevity
  short_id="${SESSION_ID:0:8}"
  output+=" | $(color_session_id "$short_id")"
fi

# Output with proper color interpretation using printf
printf "\e[0m%s\e[0m\n" "$output"

# If bumper lanes active and there's a diff tree, show it on subsequent lines
if [[ -n "$BUMPER_STATUS" && -n "$DIFF_TREE" ]]; then
  # ccstatusline technique for preserving whitespace in Claude Code status lines:
  # 1. Replace spaces with non-breaking spaces (U+00A0 = UTF-8 0xC2 0xA0)
  # 2. Prepend ANSI reset (\x1b[0m) to EACH line to override Claude Code's formatting
  while IFS= read -r line; do
    # Replace spaces with non-breaking spaces and prepend reset
    printf '\e[0m%s\n' "${line// /$'\xc2\xa0'}"
  done <<<"$DIFF_TREE"
fi

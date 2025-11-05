#!/bin/bash
# Read JSON input once
input=$(cat)

# Helper functions for common extractions
get_model_name() { echo "$input" | jq -r '.model.display_name'; }
get_current_dir() { echo "$input" | jq -r '.workspace.current_dir'; }
get_cost() { echo "$input" | jq -r '.cost.total_cost_usd'; }
get_transcript_path() { echo "$input" | jq -r '.transcript_path // ""'; }

# Color helper functions
color_model() { printf "\e[95m%s\e[0m" "$1"; }          # magenta
color_branch() { printf "\e[94m%s\e[0m" "$1"; }         # blue
color_changes() { printf "\e[32m%s\e[0m" "$1"; }        # green
color_tokens() { printf "\e[33m%s\e[0m" "$1"; }         # yellow
color_waiting() { printf "\e[36m%s\e[0m" "$1"; }        # cyan
color_cost() { printf "\e[35m%s\e[0m" "$1"; }           # magenta
color_bumper_active() { printf "\e[32m%s\e[0m" "$1"; }  # green
color_bumper_tripped() { printf "\e[31m%s\e[0m" "$1"; } # red

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
  local checkpoint_file=".git/bumper-checkpoints/session-$session_id"

  # No session file = bumper lanes not active
  [[ ! -f "$checkpoint_file" ]] && return

  # Read stop_triggered flag
  local stop_triggered=$(jq -r '.stop_triggered // false' "$checkpoint_file" 2>/dev/null)

  if [[ "$stop_triggered" == "true" ]]; then
    echo "tripped"
  else
    echo "active"
  fi
}

# Use the helpers
MODEL=$(get_model_name)
DIR=$(get_current_dir)
TOKENS=$(get_total_tokens)
BRANCH=$(get_git_branch)
CHANGES=$(get_git_changes)
COST=$(get_cost)
BUMPER_STATUS=$(get_bumper_lanes_status)

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
  if [[ "$BUMPER_STATUS" == "active" ]]; then
    output+=" | $(color_bumper_active "bumper-lanes active")"
  elif [[ "$BUMPER_STATUS" == "tripped" ]]; then
    output+=" | $(color_bumper_tripped "bumper-lanes tripped")"
  fi
fi

# Output with proper color interpretation using printf
printf "\e[0m%s\e[0m\n" "$output"

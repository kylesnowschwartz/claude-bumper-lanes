#!/usr/bin/env bash
set -euo pipefail

# user-prompt-submit.sh - UserPromptSubmit hook for /bumper-reset command
# Purpose: Watch for /bumper-reset in user prompt and execute reset-baseline.sh

# Source library functions (for potential future use)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Read hook input from stdin
input=$(cat)
prompt=$(echo "$input" | jq -r '.prompt // ""')
session_id=$(echo "$input" | jq -r '.session_id')

# Check if user typed /claude-bumper-lanes:bumper-reset
if [[ "$prompt" == *"/claude-bumper-lanes:bumper-reset"* ]]; then
  # Execute the reset baseline script with session_id
  reset_output=$("$SCRIPT_DIR/reset-baseline.sh" "$session_id" 2>&1)
  exit_code=$?

  if [[ $exit_code -eq 0 ]]; then
    # Success - inject output as additional context
    jq -n \
      --arg output "$reset_output" \
      '{
        hookSpecificOutput: {
          hookEventName: "UserPromptSubmit",
          additionalContext: $output
        }
      }'
  else
    # Error - still inject the error message
    jq -n \
      --arg output "$reset_output" \
      '{
        hookSpecificOutput: {
          hookEventName: "UserPromptSubmit",
          additionalContext: $output
        }
      }'
  fi
fi

exit 0

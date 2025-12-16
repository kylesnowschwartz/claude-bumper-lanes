#!/usr/bin/env bash
set -euo pipefail

# user-prompt-submit.sh - UserPromptSubmit hook for /bumper-reset command
# Purpose: Watch for /bumper-reset in user prompt and execute reset-baseline.sh

# Source library functions (for potential future use)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$SCRIPT_DIR/../bin"

# Read hook input from stdin
input=$(cat)
prompt=$(echo "$input" | jq -r '.prompt // ""')
session_id=$(echo "$input" | jq -r '.session_id')

# Helper function to output command result as JSON
output_command_result() {
  local output="$1"
  jq -n \
    --arg output "$output" \
    '{
      hookSpecificOutput: {
        hookEventName: "UserPromptSubmit",
        additionalContext: $output
      }
    }'
}

# Check if user typed /claude-bumper-lanes:bumper-reset
if [[ "$prompt" == *"/claude-bumper-lanes:bumper-reset"* ]]; then
  reset_output=$("$BIN_DIR/reset-baseline.sh" "$session_id" 2>&1)
  output_command_result "$reset_output"
  exit 0
fi

# Check if user typed /claude-bumper-lanes:bumper-pause
if [[ "$prompt" == *"/claude-bumper-lanes:bumper-pause"* ]]; then
  pause_output=$("$BIN_DIR/pause-baseline.sh" "$session_id" 2>&1)
  output_command_result "$pause_output"
  exit 0
fi

# Check if user typed /claude-bumper-lanes:bumper-resume
if [[ "$prompt" == *"/claude-bumper-lanes:bumper-resume"* ]]; then
  resume_output=$("$BIN_DIR/resume-baseline.sh" "$session_id" 2>&1)
  output_command_result "$resume_output"
  exit 0
fi

exit 0

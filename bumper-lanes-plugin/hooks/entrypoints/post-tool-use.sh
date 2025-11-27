#!/usr/bin/env bash
set -euo pipefail

# post-tool-use.sh - PostToolUse hook for auto-reset after git commit
# Purpose: Reset bumper-lanes baseline after successful git commits
# Hook: PostToolUse with Bash matcher
# Trigger: After Bash tool completes successfully

# Source library functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/git-state.sh"
source "$SCRIPT_DIR/../lib/state-manager.sh"

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id')
tool_name=$(echo "$input" | jq -r '.tool_name')
hook_event_name=$(echo "$input" | jq -r '.hook_event_name')
command=$(echo "$input" | jq -r '.tool_input.command // empty')

# Validate hook event (defensive check)
if [[ "$hook_event_name" != "PostToolUse" ]]; then
  exit 0
fi

# Only process Bash tool calls
if [[ "$tool_name" != "Bash" ]]; then
  exit 0
fi

# Detect git commit commands (various formats)
# Pattern: git, optional flags (like -C /path), then commit subcommand
# Matches: git commit, git -C /path commit, git --git-dir=/x commit
# Rejects: prose like "use git to commit" (non-flag words between git and commit)
if ! echo "$command" | grep -qE 'git\s+(-{1,2}[A-Za-z-]+([ =]("[^"]*"|\S+))?\s+)*commit\b'; then
  exit 0
fi

# Check if session state exists (session-only enforcement)
if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
  # No session state - not enforcing, no reset needed (fail-open)
  exit 0
fi

# Capture tree SHA from the commit that just happened
# Use HEAD^{tree} to get the tree from the commit, not current index state
# This ensures we capture exactly what was committed, not what's staged
if ! current_tree=$(git rev-parse HEAD^{tree} 2>/dev/null); then
  # Failed to get commit tree - fail open (don't break git workflow)
  exit 0
fi

# Reset baseline to current tree (committed state)
if ! reset_baseline_after_commit "$session_id" "$current_tree" 2>/dev/null; then
  # Reset failed - fail open
  exit 0
fi

# Output structured feedback for Claude Code
# PostToolUse hooks return JSON with systemMessage to inform the agent
jq -n '{
    systemMessage: "✓ Bumper lanes: Auto-reset after commit. Fresh budget: 400 pts."
  }'

exit 0

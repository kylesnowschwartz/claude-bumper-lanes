#!/bin/bash
# bumper-lanes-addon - Outputs indicator + diff tree only
# Appends to Claude Code's default status line
#
# Usage: Set as statusLine.command in ~/.claude/settings.json
# The script receives Claude status JSON via stdin and outputs:
#   1. Bumper-lanes indicator (e.g., "active (125/400 - 31%)")
#   2. Diff tree visualization

# Read JSON from stdin
input=$(cat)

# Output indicator (single line)
echo "$input" | bumper-lanes status --widget=indicator

# Output diff tree (multi-line)
echo "$input" | bumper-lanes status --widget=diff-tree

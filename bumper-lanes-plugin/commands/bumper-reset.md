---
title: "Bumper Reset"
description: "Reset the diff baseline and restore threshold budget"
usage: "/bumper-reset"
category: "bumper-lanes"
---

# `/bumper-reset` Command

## Purpose

Resets the baseline git tree to the current working directory state and restores your full threshold budget. Use this command after reviewing code changes to acknowledge them and allow Claude to continue.

## Usage

```
/bumper-reset
```

No arguments required. Command is idempotent (safe to run multiple times).

## When to Use

- After reviewing code changes when threshold block is triggered
- To checkpoint progress before large refactoring
- When you want to accept current changes and start fresh threshold tracking

## Example Output

```
✓ Baseline reset complete.

Previous baseline: 3a4b5c6 (captured 2025-11-02 20:15:00)
New baseline: 1f2e3d4 (captured 2025-11-02 20:45:30)

Changes accepted: 8 files, 287 insertions(+), 143 deletions(-) [430 lines total]

You now have a fresh diff budget of 300 lines. Pick up where we left off?
```

## Behavior

1. Captures current working tree state as new baseline
2. Computes diff statistics since old baseline (for reporting)
3. Updates session state with new baseline tree SHA
4. Prints confirmation message showing changes accepted and fresh budget

## Error Cases

- **No active session**: If `/bumper-reset` is run without an active Claude session, you'll see: "⚠ Bumper Lanes: No active session found. Baseline reset skipped."
- **Git errors**: If git operations fail, you'll see: "⚠ Bumper Lanes: Failed to reset baseline. Please try again."

## Notes

- Command does not create git commits (only updates internal baseline tracking)
- Works with both tracked and untracked files
- Safe to use even when not blocked (updates baseline anyway)
- Baseline persists until next `/bumper-reset` or session end

---

## Implementation

This command executes the reset baseline script using Claude Code's `!` syntax:

```bash
!${CLAUDE_PLUGIN_ROOT}/hooks/entrypoints/reset-baseline.sh ${CLAUDE_SESSION_ID}
```

The script receives the session ID as a command-line argument and performs the baseline reset operation.

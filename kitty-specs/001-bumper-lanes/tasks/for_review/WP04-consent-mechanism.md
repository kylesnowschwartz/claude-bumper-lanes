---
work_package_id: "WP04"
subtasks:
  - "T013"
  - "T014"
title: "Consent Mechanism"
phase: "Phase 1 - Core MVP"
lane: "for_review"
assignee: ""
agent: "claude"
shell_pid: "37042"
history:
  - timestamp: "2025-11-02T20:56:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
  - timestamp: "2025-11-02T21:31:00Z"
    lane: "planned"
    agent: "claude"
    shell_pid: ""
    action: "Rewritten after user corrections (! syntax command, deleted UserPromptSubmit hook, reset-baseline.sh script)"
---

# Work Package Prompt: WP04 – Consent Mechanism

## Objectives & Success Criteria

- Implement `/bumper-reset` command that resets baseline via bash script execution
- Reset baseline to current working tree state when user provides consent
- Show confirmation message with changes accepted and fresh threshold budget
- Support idempotent resets (safe to run multiple times)
- Use slash command `!` syntax to directly execute bash script (no hook required)

**Success Metric**: Trigger threshold block, execute `/bumper-reset`, verify baseline resets, block is lifted, and Claude can continue with fresh 300-line budget. Confirmation message shows old/new baseline and accepted changes.

## Context & Constraints

**Prerequisites**:
- WP01 complete (plugin structure)
- WP02 complete (library functions)
- WP03 complete (session state must exist for reset to work)

**Supporting Documents**:
- `kitty-specs/001-bumper-lanes/spec.md` - User Story 2 (P1 - Consent and Reset)
- `kitty-specs/001-bumper-lanes/quickstart.md` - Example `/bumper-reset` usage and output
- `kitty-specs/001-bumper-lanes/data-model.md` - Session State entity (baseline_tree update)

**Architectural Decisions**:
- **Command approach**: Use slash command `!` syntax to execute bash script directly
- **Script location**: `bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh`
- **SessionId extraction**: Pass as command-line argument from command markdown file
- **Idempotency**: Running `/bumper-reset` multiple times is safe (updates baseline each time)
- **Missing session**: Print error message if no active session found (graceful failure)
- **Pick up where left off**: Confirmation message offers to continue coding

## Subtasks & Detailed Guidance

### Subtask T013 – Implement `bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh`

**Purpose**: Reset baseline tree to current working tree state, report accepted changes, update session state.

**Steps**:
1. Create `bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh` file
2. Add shebang: `#!/usr/bin/env bash`
3. Add strict mode: `set -euo pipefail`
4. Source required libraries:
   ```bash
   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
   source "$SCRIPT_DIR/../lib/git-state.sh"
   source "$SCRIPT_DIR/../lib/state-manager.sh"
   source "$SCRIPT_DIR/../lib/threshold.sh"
   ```
5. Read command-line argument (sessionId passed from command):
   ```bash
   session_id=${1:-}

   if [[ -z "$session_id" ]]; then
     echo "⚠ Bumper Lanes: Error - No session ID provided"
     exit 1
   fi
   ```
6. Load session state:
   ```bash
   if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
     # No active session - print error message
     echo "⚠ Bumper Lanes: No active session found. Baseline reset skipped."
     exit 0
   fi

   old_baseline=$(echo "$session_state" | jq -r '.baseline_tree')
   threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')
   created_at=$(echo "$session_state" | jq -r '.created_at')
   ```
7. Compute final diff stats (for reporting accepted changes):
   ```bash
   current_tree=$(capture_tree)
   if [[ -z "$current_tree" ]]; then
     echo "⚠ Bumper Lanes: Failed to reset baseline. Please try again."
     exit 1
   fi

   diff_output=$(compute_diff "$old_baseline" "$current_tree")
   diff_stats=$(parse_diff_stats "$diff_output")
   total_lines=$(echo "$diff_stats" | jq -r '.total_lines_changed')
   ```
8. Update session state with new baseline:
   ```bash
   new_baseline="$current_tree"
   write_session_state "$session_id" "$new_baseline"
   ```
9. Build confirmation message:
   ```bash
   # Format timestamps for display
   old_timestamp=$(date -d "$created_at" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "$created_at")
   new_timestamp=$(date -u "+%Y-%m-%d %H:%M:%S")

   # Extract stats for message
   files_changed=$(echo "$diff_stats" | jq -r '.files_changed')
   lines_added=$(echo "$diff_stats" | jq -r '.lines_added')
   lines_deleted=$(echo "$diff_stats" | jq -r '.lines_deleted')

   # Truncate SHAs for display
   old_baseline_short="${old_baseline:0:7}"
   new_baseline_short="${new_baseline:0:7}"

   # Build multi-line confirmation message
   cat <<EOF
✓ Baseline reset complete.

Previous baseline: $old_baseline_short (captured $old_timestamp)
New baseline: $new_baseline_short (captured $new_timestamp)

Changes accepted: $files_changed files, $lines_added insertions(+), $lines_deleted deletions(-) [$total_lines lines total]

You now have a fresh diff budget of $threshold_limit lines. Pick up where we left off?
EOF
   ```
10. Exit successfully:
    ```bash
    exit 0
    ```
11. Make executable: `chmod +x bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh`

**Files**: `bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh`

**Parallel?**: Can be implemented alongside T014 (command spec documentation).

**Notes**:
- Script is NOT a hook - it's called directly via slash command `!` syntax
- SessionId passed as first command-line argument (`$1`)
- Confirmation message goes to stdout (user sees it directly)
- Error messages use "⚠ Bumper Lanes:" prefix for clarity
- Idempotent: Safe to run multiple times (updates baseline each time)
- "Pick up where we left off?" prompts user to continue coding

### Subtask T014 – Create `/bumper-reset` command spec

**Purpose**: Document `/bumper-reset` slash command with usage, behavior, and execution via `!` syntax.

**Steps**:
1. Create `bumper-lanes-plugin/commands/bumper-reset.md` file
2. Add frontmatter (YAML metadata):
   ```yaml
   ---
   title: "Bumper Reset"
   description: "Reset the diff baseline and restore threshold budget"
   usage: "/bumper-reset"
   category: "bumper-lanes"
   ---
   ```
3. Write command documentation with `!` syntax execution:
   ````markdown
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
   ````
4. Validate markdown syntax

**Files**: `bumper-lanes-plugin/commands/bumper-reset.md`

**Parallel?**: Can be written alongside T013 (script implementation).

**Notes**:
- Frontmatter format follows Claude Code slash command conventions
- `category: "bumper-lanes"` groups command with plugin in `/help` output
- Usage section shows command syntax (no arguments)
- Example output matches confirmation message from T013
- Implementation section documents `!` syntax execution
- `${CLAUDE_SESSION_ID}` is a Claude Code environment variable containing the session UUID
- Error cases documented for troubleshooting
- "Notes" section clarifies command behavior (no commits, works with untracked files, etc.)

## Test Strategy

**No automated tests required** - manual end-to-end test sufficient.

**Manual Test Scenario** (from spec.md User Story 2):
1. Complete WP03 test (trigger threshold block)
2. With Claude blocked, run `/bumper-reset` command
3. Verify confirmation message appears:
   - Shows old and new baseline SHAs (truncated to 7 chars)
   - Shows changes accepted (file count, insertions, deletions)
   - States fresh threshold budget (300 lines)
   - Asks "Pick up where we left off?"
4. Attempt to continue with Claude - verify block is lifted
5. Make new changes (under 300 lines) - verify Claude continues normally
6. Make cumulative changes exceeding 300 lines - verify new block triggered (from new baseline)

**Edge Case Tests**:
- Run `/bumper-reset` when not blocked - verify updates baseline without error
- Run `/bumper-reset` with no active session - verify error message printed
- Run `/bumper-reset` multiple times in a row - verify idempotent (updates baseline each time)

**Validation Commands**:
```bash
# Check baseline before reset
cat .git/bumper-checkpoints/session-{sessionId} | jq .baseline_tree

# (Run /bumper-reset)

# Check baseline after reset (should be different)
cat .git/bumper-checkpoints/session-{sessionId} | jq .baseline_tree

# Manually test script
bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh {sessionId}
```

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| SessionId not passed to script | `${CLAUDE_SESSION_ID}` is standard Claude Code environment variable |
| Command not recognized | Slash commands discovered by directory convention (commands/ folder) |
| Confirmation message not visible | Script prints to stdout (user sees directly, not via hook context injection) |
| User resets without reviewing code | Document command purpose clearly, accept risk for MVP |
| Git operations fail mid-reset | Print error message, leave old baseline intact (fail safe) |

## Definition of Done Checklist

- [ ] `bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh` implemented with baseline reset logic
- [ ] `bumper-lanes-plugin/commands/bumper-reset.md` created with usage documentation and `!` syntax execution
- [ ] Script executable (`chmod +x reset-baseline.sh`)
- [ ] Manual test scenario passes: threshold block lifted after reset
- [ ] Confirmation message format matches quickstart.md example
- [ ] Edge case tests pass (no session, idempotent resets)
- [ ] `/bumper-reset` appears in `/help` command list
- [ ] `tasks.md` WP04 checkbox marked complete

## Review Guidance

**Acceptance Checkpoints**:
1. reset-baseline.sh is standalone script (NOT a hook)
2. Script receives sessionId as command-line argument ($1)
3. Script updates session state with new baseline tree SHA
4. Confirmation message includes old baseline, new baseline, changes accepted, and fresh budget
5. Confirmation message ends with "Pick up where we left off?"
6. Error cases print friendly messages (no active session, git failure)
7. Command spec documentation includes `!` syntax execution in Implementation section
8. `/bumper-reset` command is idempotent (safe to run multiple times)

**What to verify**:
- No UserPromptSubmit hook registration in hooks.json
- Script location: `bumper-lanes-plugin/hooks/entrypoints/reset-baseline.sh`
- Command markdown uses `${CLAUDE_SESSION_ID}` environment variable
- Confirmation message is multi-line and human-readable
- Error messages use "⚠ Bumper Lanes:" prefix for clarity
- Frontmatter in command spec follows Claude Code conventions
- Script exits 0 on success (no throw/exit 1 unless fatal error)

## Activity Log

- 2025-11-02T20:56:00Z – system – lane=planned – Prompt created via /spec-kitty.tasks.
- 2025-11-02T21:31:00Z – claude – lane=planned – Rewritten after user corrections (! syntax command, deleted UserPromptSubmit hook, reset-baseline.sh script).
- 2025-11-02T09:45:31Z – claude – shell_pid=37042 – lane=doing – Started WP04: Consent Mechanism
- 2025-11-02T09:49:47Z – claude – shell_pid=37042 – lane=doing – Completed T013 (reset-baseline.sh) and T014 (bumper-reset.md command spec)
- 2025-11-02T09:52:11Z – claude – shell_pid=37042 – lane=for_review – Ready for review: Consent mechanism implemented

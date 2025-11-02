---
work_package_id: "WP03"
subtasks:
  - "T009"
  - "T010"
title: "Automatic Threshold Enforcement"
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
    action: "Rewritten after user corrections (sessionId extraction, deleted session-end.sh, no SubagentStop)"
---

# Work Package Prompt: WP03 – Automatic Threshold Enforcement

## Objectives & Success Criteria

- Implement hook lifecycle: SessionStart (baseline capture) → Stop (threshold check)
- Block agent execution when cumulative diff exceeds 300 lines (hardcoded threshold)
- Provide clear block messages with diff statistics and instructions
- Maintain fail-open error handling (plugin never breaks Claude workflow)
- Support concurrent sessions via sessionId-based state isolation

**Success Metric**: Start Claude session, make incremental changes across multiple files until 300-line threshold exceeded, verify block message appears with accurate stats and `/bumper-reset` instruction.

## Context & Constraints

**Prerequisites**:
- WP01 complete (plugin structure and registration)
- WP02 complete (library functions available)

**Supporting Documents**:
- `kitty-specs/001-bumper-lanes/spec.md` - User Story 1 (P1 - Automatic Enforcement)
- `kitty-specs/001-bumper-lanes/contracts/session-start-hook.json` - SessionStart hook behavior spec
- `kitty-specs/001-bumper-lanes/contracts/stop-hook.json` - Stop hook behavior spec
- `kitty-specs/001-bumper-lanes/data-model.md` - Session State entity, Block Event entity

**Architectural Decisions**:
- **Fail-open strategy**: If any hook errors, allow operation to proceed (never block Claude on plugin failure)
- **Block response format**: `{\"decision\": \"block\", \"reason\": \"...\"}` on stdout
- **Non-git repos**: Detect via `git rev-parse --git-dir`, disable plugin gracefully (exit 0)
- **Missing baseline**: Treat current state as new baseline, allow stop (fail open)
- **SessionId extraction**: Read from hook stdin JSON field `.sessionId`
- **No SubagentStop**: Subagents excluded from MVP (unpredictable behavior)
- **No cleanup**: Session state files persist indefinitely (cleanup in v2)

## Subtasks & Detailed Guidance

### Subtask T009 – Implement `bumper-lanes-plugin/hooks/entrypoints/session-start.sh`

**Purpose**: Capture baseline tree SHA on session initialization, write session state.

**Steps**:
1. Create `bumper-lanes-plugin/hooks/entrypoints/session-start.sh` file
2. Add shebang: `#!/usr/bin/env bash`
3. Add strict mode: `set -euo pipefail`
4. Source required libraries:
   ```bash
   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
   source "$SCRIPT_DIR/../lib/git-state.sh"
   source "$SCRIPT_DIR/../lib/state-manager.sh"
   ```
5. Read stdin JSON (hook input from Claude Code):
   ```bash
   input=$(cat)
   working_dir=$(echo "$input" | jq -r '.working_directory')
   session_id=$(echo "$input" | jq -r '.sessionId')

   cd "$working_dir" || exit 0
   ```
6. Check if git repository:
   ```bash
   if ! git rev-parse --git-dir &>/dev/null; then
     # Not a git repo - disable plugin gracefully
     exit 0
   fi
   ```
7. Capture baseline tree:
   ```bash
   baseline_tree=$(capture_tree)
   if [[ -z "$baseline_tree" ]]; then
     echo "ERROR: Failed to capture baseline tree" >&2
     exit 0  # Fail open
   fi
   ```
8. Write session state:
   ```bash
   write_session_state "$session_id" "$baseline_tree"
   ```
9. Exit successfully:
   ```bash
   exit 0
   ```
10. Make executable: `chmod +x bumper-lanes-plugin/hooks/entrypoints/session-start.sh`

**Files**: `bumper-lanes-plugin/hooks/entrypoints/session-start.sh`

**Parallel?**: No - must run before Stop hooks can function (establishes baseline).

**Notes**:
- Hook receives JSON on stdin with `working_directory` and `sessionId` fields
- `jq -r` extracts raw string values (no quotes)
- Non-git repo detection exits 0 (success) to avoid blocking session start
- Session state file: `.git/bumper-checkpoints/session-{sessionId}` (UUID-based)
- Threshold hardcoded to 300 lines in state-manager.sh
- No environment variable export needed (sessionId passed via stdin for each hook)

### Subtask T010 – Implement `bumper-lanes-plugin/hooks/entrypoints/stop.sh`

**Purpose**: Check diff threshold when agent stops, return block decision if threshold exceeded.

**Steps**:
1. Create `bumper-lanes-plugin/hooks/entrypoints/stop.sh` file
2. Add shebang and strict mode
3. Source required libraries:
   ```bash
   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
   source "$SCRIPT_DIR/../lib/git-state.sh"
   source "$SCRIPT_DIR/../lib/state-manager.sh"
   source "$SCRIPT_DIR/../lib/threshold.sh"
   ```
4. Read stdin JSON:
   ```bash
   input=$(cat)
   hook_name=$(echo "$input" | jq -r '.hook_name')
   working_dir=$(echo "$input" | jq -r '.working_directory')
   session_id=$(echo "$input" | jq -r '.sessionId')

   cd "$working_dir" || exit 0
   ```
5. Load session state:
   ```bash
   if ! session_state=$(read_session_state "$session_id" 2>/dev/null); then
     # No baseline - allow stop (fail open)
     echo "null"
     exit 0
   fi

   baseline_tree=$(echo "$session_state" | jq -r '.baseline_tree')
   threshold_limit=$(echo "$session_state" | jq -r '.threshold_limit')
   ```
6. Capture current working tree:
   ```bash
   current_tree=$(capture_tree)
   if [[ -z "$current_tree" ]]; then
     echo "ERROR: Failed to capture current tree" >&2
     echo "null"
     exit 0  # Fail open
   fi
   ```
7. Compute diff statistics:
   ```bash
   diff_output=$(compute_diff "$baseline_tree" "$current_tree")
   ```
8. Calculate threshold and check limit:
   ```bash
   total_lines=$(calculate_threshold "$diff_output")

   if [[ $total_lines -le $threshold_limit ]]; then
     # Under threshold - allow stop
     echo "null"
     exit 0
   fi
   ```
9. Build block response:
   ```bash
   # Parse diff stats for detailed reporting
   diff_stats=$(parse_diff_stats "$diff_output")

   files_changed=$(echo "$diff_stats" | jq -r '.files_changed')
   lines_added=$(echo "$diff_stats" | jq -r '.lines_added')
   lines_deleted=$(echo "$diff_stats" | jq -r '.lines_deleted')

   threshold_pct=$(awk "BEGIN {printf \"%.1f\", ($total_lines / $threshold_limit) * 100}")

   # Build reason message
   reason="⚠ Diff threshold exceeded: $total_lines/$threshold_limit lines changed (${threshold_pct}%).

Changes:
  $files_changed files changed, $lines_added insertions(+), $lines_deleted deletions(-)

Review your changes and run /bumper-reset to continue."

   # Output block decision
   jq -n \
     --arg decision "block" \
     --arg reason "$reason" \
     --argjson diff_stats "$diff_stats" \
     --argjson threshold_limit "$threshold_limit" \
     --argjson threshold_percentage "$threshold_pct" \
     '{
       decision: $decision,
       reason: $reason,
       diff_stats: ($diff_stats + {threshold_limit: $threshold_limit, threshold_percentage: $threshold_percentage})
     }'

   exit 0
   ```
10. Make executable: `chmod +x bumper-lanes-plugin/hooks/entrypoints/stop.sh`

**Files**: `bumper-lanes-plugin/hooks/entrypoints/stop.sh`

**Parallel?**: No - sequential logic (load state → compute diff → check threshold → return decision).

**Notes**:
- Hook receives JSON with `hook_name`, `working_directory`, and `sessionId`
- Missing session state → output null and exit 0 (allow stop, fail open)
- Threshold check: `total_lines > threshold_limit` (300) triggers block
- Block response includes structured diff stats for debugging
- `jq -n` constructs JSON from scratch with `--arg` and `--argjson` flags
- Reason message includes explicit `/bumper-reset` instruction
- Error handling: Any git failure → output null, exit 0 (fail open)

## Test Strategy

**No automated tests required** - manual end-to-end test sufficient.

**Manual Test Scenario** (from spec.md User Story 1):
1. Install plugin in test repository
2. Start Claude Code session
3. Ask Claude to make small changes (under 300 lines)
4. Verify Claude stops normally (no block message)
5. Ask Claude to make more changes (cumulative total exceeds 300 lines)
6. Verify block message appears with:
   - Threshold exceeded indicator (e.g., "430/300 lines changed (143%)")
   - File change summary (e.g., "8 files changed, 287 insertions(+), 143 deletions(-)")
   - Instruction to run `/bumper-reset`
7. Attempt to continue without reset - verify block persists

**Validation Commands**:
```bash
# Check baseline captured
ls .git/bumper-checkpoints/session-*

# Check session state content (replace {sessionId} with actual value)
cat .git/bumper-checkpoints/session-{sessionId} | jq .

# Manually trigger stop.sh to test threshold
echo '{"hook_name":"Stop","working_directory":"'$(pwd)'","sessionId":"test-123"}' | \
  bumper-lanes-plugin/hooks/entrypoints/stop.sh
```

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Hook errors break Claude workflow | Fail-open strategy: all errors output null, exit 0 |
| Baseline not captured on session start | Stop hook checks for missing state, allows stop with null output |
| Git command failures in large repos | Set timeout (5s) on git operations, fail open on timeout |
| False positives (blocks reasonable changes) | Threshold 300 lines tuned for balance, user can review/reset |
| SessionId extraction fails | Hook stdin always provides `.sessionId` field (Claude Code standard) |

## Definition of Done Checklist

- [ ] `bumper-lanes-plugin/hooks/entrypoints/session-start.sh` implemented with baseline capture and state write
- [ ] `bumper-lanes-plugin/hooks/entrypoints/stop.sh` implemented with threshold check and block logic
- [ ] Both scripts executable (`chmod +x`)
- [ ] Manual test scenario passes: threshold block triggered with accurate stats
- [ ] Block message includes diff statistics and `/bumper-reset` instruction
- [ ] SessionId-based state files created (not PID-based)
- [ ] `tasks.md` WP03 checkbox marked complete

## Review Guidance

**Acceptance Checkpoints**:
1. SessionStart hook creates `.git/bumper-checkpoints/session-{sessionId}` file with valid JSON
2. SessionId extracted from hook stdin field `.sessionId`
3. Stop hook outputs null when under threshold (allows stop)
4. Stop hook outputs block JSON when over threshold (prevents stop)
5. Block message format matches contract (decision="block", reason includes stats)
6. No session-end.sh hook (no cleanup for MVP)
7. All hooks use fail-open error handling (never exit non-zero on internal errors)

**What to verify**:
- Session state JSON matches schema: `{"session_id", "baseline_tree", "created_at", "threshold_limit", "repo_path"}`
- Baseline tree SHA is valid 40-character hex string
- Diff statistics are accurate (compare with manual `git diff` output)
- Block message is human-readable and actionable
- Non-git repos disable plugin gracefully (no error messages)
- No SubagentStop registration in hooks.json

## Activity Log

- 2025-11-02T20:56:00Z – system – lane=planned – Prompt created via /spec-kitty.tasks.
- 2025-11-02T21:31:00Z – claude – lane=planned – Rewritten after user corrections (sessionId extraction, deleted session-end.sh, no SubagentStop).
- 2025-11-02T09:27:47Z – claude – shell_pid=37042 – lane=doing – Started WP03: Automatic Threshold Enforcement
- 2025-11-02T09:45:17Z – claude – shell_pid=37042 – lane=for_review – Completed WP03: Both hooks implemented with empirically verified schemas

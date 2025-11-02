---
work_package_id: "WP02"
subtasks:
  - "T005"
  - "T007"
  - "T008"
title: "Shared Hook Utilities"
phase: "Phase 0 - Foundational"
lane: "doing"
assignee: ""
agent: "claude"
shell_pid: "21511"
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
    action: "Rewritten after user corrections (deleted config-loader.sh, sessionId-based state)"
---

# Work Package Prompt: WP02 – Shared Hook Utilities

## Objectives & Success Criteria

- Implement reusable bash library functions for git tree operations, session state management, and threshold calculation
- Each library module exports well-defined functions with predictable error handling (fail-open strategy)
- Libraries follow bash best practices: shebang, set -euo pipefail, function documentation
- All libraries can be sourced independently without side effects
- No configuration system (threshold hardcoded to 300 lines for MVP)

**Success Metric**: Each library script sources successfully, exported functions are callable, and smoke tests (manual invocations) return expected output.

## Context & Constraints

**Prerequisites**: WP01 must be complete (directory structure must exist: `bumper-lanes-plugin/hooks/lib/`).

**Supporting Documents**:
- `kitty-specs/001-bumper-lanes/research.md` - Q1 (tree capture method), Q2 (diff computation), Q4 (sessionId-based isolation), Q5 (threshold metric)
- `kitty-specs/001-bumper-lanes/data-model.md` - Session State entity (storage format), Diff Statistics entity (computed fields)
- `kitty-specs/001-bumper-lanes/contracts/session-start-hook.json` - Baseline capture behavior
- `kitty-specs/001-bumper-lanes/contracts/stop-hook.json` - Diff computation behavior

**Architectural Decisions**:
- **Fail-open strategy**: If any git operation fails, log to stderr and return safe defaults (allow hook to proceed)
- **No global state**: Libraries should not modify environment or persist state without explicit function calls
- **SessionId isolation**: Session state files use Claude's conversation UUID for concurrent session support
- **jq dependency**: Required for JSON parsing (document this dependency in README)
- **No configuration system**: Threshold hardcoded to 300 lines (config system for v2)

## Subtasks & Detailed Guidance

### Subtask T005 – Implement `bumper-lanes-plugin/hooks/lib/git-state.sh`

**Purpose**: Provide functions for capturing working tree as git tree objects and computing diff statistics between two trees.

**Functions to implement**:
1. `capture_tree()` - Captures current working tree (including untracked files) as a git tree object
2. `compute_diff()` - Computes diff statistics between two tree SHAs

**Steps**:
1. Create `bumper-lanes-plugin/hooks/lib/git-state.sh` file
2. Add shebang: `#!/usr/bin/env bash`
3. Add strict mode: `set -euo pipefail`
4. Implement `capture_tree()`:
   ```bash
   capture_tree() {
     # Uses temporary index approach to avoid modifying actual staging area
     local tmp_index=$(mktemp)
     trap "rm -f $tmp_index" EXIT

     export GIT_INDEX_FILE="$tmp_index"
     git add -u . 2>/dev/null || true
     git ls-files --others --exclude-standard 2>/dev/null | xargs -r git add 2>/dev/null || true
     local tree_sha=$(git write-tree 2>/dev/null)
     unset GIT_INDEX_FILE

     if [[ -z "$tree_sha" ]]; then
       echo "ERROR: Failed to capture tree" >&2
       return 1
     fi

     echo "$tree_sha"
     return 0
   }
   ```
5. Implement `compute_diff()`:
   ```bash
   compute_diff() {
     local baseline_tree=$1
     local current_tree=$2

     if [[ -z "$baseline_tree" ]] || [[ -z "$current_tree" ]]; then
       echo "ERROR: compute_diff requires two tree SHAs" >&2
       return 1
     fi

     # Use git diff-tree to compare trees
     local diff_output=$(git diff-tree --shortstat "$baseline_tree" "$current_tree" 2>/dev/null)

     if [[ -z "$diff_output" ]]; then
       # No changes or error - return zero stats
       echo "0 files changed, 0 insertions(+), 0 deletions(-)"
       return 0
     fi

     echo "$diff_output"
     return 0
   }
   ```
6. Add function documentation (comments above each function explaining parameters and return values)
7. Make executable: `chmod +x bumper-lanes-plugin/hooks/lib/git-state.sh`

**Files**: `bumper-lanes-plugin/hooks/lib/git-state.sh`

**Parallel?**: Yes - can proceed alongside T007-T008 (independent module).

**Notes**:
- `capture_tree()` uses `GIT_INDEX_FILE` to create temporary index without affecting real staging area
- `trap` ensures temporary index file is cleaned up even if script exits early
- `git add -u` adds tracked file changes, `git ls-files --others` adds untracked files
- `xargs -r` prevents error if no untracked files exist
- `compute_diff()` output format: "N files changed, X insertions(+), Y deletions(-)"
- If git commands fail, return sensible defaults (zero stats) to fail open

### Subtask T007 – Implement `bumper-lanes-plugin/hooks/lib/state-manager.sh`

**Purpose**: Manage sessionId-based session state files with read/write operations. No cleanup logic for MVP.

**Functions to implement**:
1. `write_session_state()` - Writes session state JSON to `.git/bumper-checkpoints/session-{sessionId}`
2. `read_session_state()` - Reads session state JSON from `.git/bumper-checkpoints/session-{sessionId}`

**Steps**:
1. Create `bumper-lanes-plugin/hooks/lib/state-manager.sh` file
2. Add shebang: `#!/usr/bin/env bash`
3. Add strict mode: `set -euo pipefail`
4. Implement `write_session_state()`:
   ```bash
   write_session_state() {
     local session_id=$1
     local baseline_tree=$2
     local repo_path=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
     local checkpoint_dir=".git/bumper-checkpoints"

     mkdir -p "$checkpoint_dir" 2>/dev/null || true

     local state_file="$checkpoint_dir/session-$session_id"
     local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

     cat > "$state_file" <<EOF
{
  "session_id": "$session_id",
  "baseline_tree": "$baseline_tree",
  "created_at": "$timestamp",
  "threshold_limit": 300,
  "repo_path": "$repo_path"
}
EOF

     return 0
   }
   ```
5. Implement `read_session_state()`:
   ```bash
   read_session_state() {
     local session_id=$1
     local checkpoint_dir=".git/bumper-checkpoints"
     local state_file="$checkpoint_dir/session-$session_id"

     if [[ ! -f "$state_file" ]]; then
       echo "ERROR: No session state found for session $session_id" >&2
       return 1
     fi

     cat "$state_file"
     return 0
   }
   ```
6. Add function documentation
7. Make executable: `chmod +x bumper-lanes-plugin/hooks/lib/state-manager.sh`

**Files**: `bumper-lanes-plugin/hooks/lib/state-manager.sh`

**Parallel?**: Yes - can proceed alongside T005, T008.

**Notes**:
- `.git/bumper-checkpoints/` directory created automatically if missing
- Session state filename format: `session-{sessionId}` (e.g., `session-a1b2c3d4-e5f6-7890-abcd-ef1234567890`)
- `session_id` is extracted from hook stdin JSON field `.sessionId`
- Threshold hardcoded to 300 for MVP (no config system)
- No cleanup logic for MVP - files persist until manually deleted (cleanup in v2)
- State file is JSON for easy parsing with `jq`

### Subtask T008 – Implement `bumper-lanes-plugin/hooks/lib/threshold.sh`

**Purpose**: Calculate threshold value from diff statistics using simple line-count metric (hardcoded).

**Functions to implement**:
1. `calculate_threshold()` - Parses git diff-tree output and returns total lines changed
2. `parse_diff_stats()` - Parses diff output into structured JSON for reporting

**Steps**:
1. Create `bumper-lanes-plugin/hooks/lib/threshold.sh` file
2. Add shebang: `#!/usr/bin/env bash`
3. Add strict mode: `set -euo pipefail`
4. Implement `calculate_threshold()`:
   ```bash
   calculate_threshold() {
     local diff_output=$1

     # Parse git diff-tree --shortstat output
     # Format: "N files changed, X insertions(+), Y deletions(-)"

     local insertions=0
     local deletions=0

     if [[ "$diff_output" =~ ([0-9]+)\ insertion ]]; then
       insertions=${BASH_REMATCH[1]}
     fi

     if [[ "$diff_output" =~ ([0-9]+)\ deletion ]]; then
       deletions=${BASH_REMATCH[1]}
     fi

     # Simple line count metric: additions + deletions
     local total=$((insertions + deletions))

     echo "$total"
     return 0
   }
   ```
5. Implement `parse_diff_stats()`:
   ```bash
   parse_diff_stats() {
     local diff_output=$1

     local files_changed=0
     local insertions=0
     local deletions=0

     if [[ "$diff_output" =~ ([0-9]+)\ file ]]; then
       files_changed=${BASH_REMATCH[1]}
     fi

     if [[ "$diff_output" =~ ([0-9]+)\ insertion ]]; then
       insertions=${BASH_REMATCH[1]}
     fi

     if [[ "$diff_output" =~ ([0-9]+)\ deletion ]]; then
       deletions=${BASH_REMATCH[1]}
     fi

     local total=$((insertions + deletions))

     # Return JSON for structured consumption
     echo "{\"files_changed\":$files_changed,\"lines_added\":$insertions,\"lines_deleted\":$deletions,\"total_lines_changed\":$total}"
     return 0
   }
   ```
6. Add function documentation
7. Make executable: `chmod +x bumper-lanes-plugin/hooks/lib/threshold.sh`

**Files**: `bumper-lanes-plugin/hooks/lib/threshold.sh`

**Parallel?**: Yes - can proceed alongside T005, T007.

**Notes**:
- Regex matching extracts numeric values from git diff-tree output
- `BASH_REMATCH[1]` captures first parenthesized group in regex
- Simple line count metric: `total = additions + deletions` (hardcoded for MVP)
- Future v2 can extend with weighted metrics and configuration
- `parse_diff_stats()` returns JSON for use in block response messages
- Threshold limit hardcoded to 300 in state-manager.sh (no config file)

## Test Strategy

**No automated tests required** - manual smoke testing sufficient.

**Manual Smoke Tests**:
1. Source each library: `source bumper-lanes-plugin/hooks/lib/git-state.sh`
2. Test `capture_tree()`: Should return 40-character SHA-1
3. Test `compute_diff()`: Should return diff-tree output format
4. Test `write_session_state()` and `read_session_state()`: Should round-trip session data
5. Test `calculate_threshold()` with sample diff output: Should return total lines

**Example Test Session**:
```bash
cd /path/to/repo
source bumper-lanes-plugin/hooks/lib/git-state.sh
tree=$(capture_tree)
echo "Captured tree: $tree"

source bumper-lanes-plugin/hooks/lib/state-manager.sh
write_session_state "test-session-123" "$tree"
state=$(read_session_state "test-session-123")
echo "Session state: $state"

source bumper-lanes-plugin/hooks/lib/threshold.sh
diff_output="8 files changed, 287 insertions(+), 143 deletions(-)"
total=$(calculate_threshold "$diff_output")
echo "Total lines: $total"  # Should output: 430
```

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Git commands fail in non-git repo | Check `git rev-parse --git-dir` before calling functions |
| Missing jq dependency | Document in README prerequisites, check during plugin validation |
| Temporary index file leaks | Use `trap` to ensure cleanup even on early exit |
| SessionId extraction fails | Hook stdin always provides `.sessionId` field (Claude Code standard) |
| State files persist indefinitely | Accept for MVP, implement cleanup in v2 |

## Definition of Done Checklist

- [ ] `bumper-lanes-plugin/hooks/lib/git-state.sh` created with capture_tree() and compute_diff()
- [ ] `bumper-lanes-plugin/hooks/lib/state-manager.sh` created with write/read functions (no cleanup)
- [ ] `bumper-lanes-plugin/hooks/lib/threshold.sh` created with calculate_threshold() and parse_diff_stats()
- [ ] All three scripts have executable permissions (`chmod +x`)
- [ ] All functions have documentation comments
- [ ] Manual smoke tests pass for each library
- [ ] Threshold hardcoded to 300 (no config system)
- [ ] `tasks.md` WP02 checkbox marked complete

## Review Guidance

**Acceptance Checkpoints**:
1. Each library sources without errors
2. Functions follow fail-open strategy (return safe defaults on error, log to stderr)
3. No global state modifications (no environment variables set without explicit calls)
4. Session state uses sessionId-based filenames (`session-{sessionId}`)
5. No config loader (threshold hardcoded to 300)
6. Threshold calculation uses simple line-count metric (additions + deletions)

**What to verify**:
- Shebang line present: `#!/usr/bin/env bash`
- Strict mode enabled: `set -euo pipefail`
- Functions return predictable values (not undefined behavior)
- Error messages go to stderr (`>&2`), not stdout
- JSON output is valid (can be parsed by `jq`)
- No config-loader.sh file (deleted for MVP)

## Activity Log

- 2025-11-02T20:56:00Z – system – lane=planned – Prompt created via /spec-kitty.tasks.
- 2025-11-02T21:31:00Z – claude – lane=planned – Rewritten after user corrections (deleted config-loader.sh, sessionId-based state).
- 2025-11-02T09:07:56Z – claude – shell_pid=21511 – lane=doing – Started WP02: Shared Hook Utilities

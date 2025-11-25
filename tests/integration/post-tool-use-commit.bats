# post-tool-use-commit.bats - PostToolUse Auto-Reset After Git Commit Test Suite
#
# Purpose: Verify automatic baseline reset after successful git commits
# Hook: PostToolUse with Bash matcher
# Feature: Auto-reset bumper-lanes after committing reviewed code
#
# Test Coverage:
# - Commit detection (git commit vs other git commands)
# - Baseline reset after commit
# - Session-only enforcement (no reset without session state)
# - Uncommitted diff remains tracked after partial commits
# - State updates (baseline_tree, accumulated_score, stop_triggered flag)

# bats file_tags=integration,post-tool-use,auto-reset,commit

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'
load '../test_helper/bats-file/load'

# Load custom helpers
load '../test_helper/git-test-helpers'
load '../test_helper/threshold-helpers'
load '../test_helper/json-assertions'

# Hook script under test
POST_TOOL_USE_HOOK="$BATS_TEST_DIRNAME/../../bumper-lanes-plugin/hooks/entrypoints/post-tool-use.sh"

# Setup: Create git test repo and session state before each test
setup() {
  setup_git_test_repo

  # Create session state directory
  SESSION_ID="test-session-123"
  SESSION_STATE_DIR="$TEST_REPO/.git/bumper-checkpoints"
  SESSION_STATE_FILE="$SESSION_STATE_DIR/session-$SESSION_ID"
  mkdir -p "$SESSION_STATE_DIR"
}

# Teardown: Clean up git repo after each test
teardown() {
  cleanup_git_test_repo
}

# Helper: Create session state with given parameters
create_session_state() {
  local baseline_tree="$1"
  local threshold_limit="${2:-400}"
  local stop_triggered="${3:-false}"
  local accumulated_score="${4:-0}"
  local previous_tree="${5:-$baseline_tree}"

  jq -n \
    --arg baseline_tree "$baseline_tree" \
    --argjson threshold_limit "$threshold_limit" \
    --argjson stop_triggered "$stop_triggered" \
    --argjson accumulated_score "$accumulated_score" \
    --arg previous_tree "$previous_tree" \
    '{
      baseline_tree: $baseline_tree,
      threshold_limit: $threshold_limit,
      stop_triggered: $stop_triggered,
      accumulated_score: $accumulated_score,
      previous_tree: $previous_tree
    }' > "$SESSION_STATE_FILE"
}

# Helper: Create PostToolUse hook input JSON for Bash tool
create_post_tool_use_input() {
  local command="$1"
  local exit_code="${2:-0}"

  jq -n \
    --arg session_id "$SESSION_ID" \
    --arg cwd "$TEST_REPO" \
    --arg command "$command" \
    --argjson exit_code "$exit_code" \
    '{
      session_id: $session_id,
      transcript_path: "/tmp/transcript.jsonl",
      cwd: $cwd,
      permission_mode: "default",
      hook_event_name: "PostToolUse",
      tool_name: "Bash",
      tool_input: {
        command: $command
      },
      tool_response: {
        exit_code: $exit_code,
        stdout: "",
        stderr: ""
      }
    }'
}

# Test 1: Should detect git commit commands
# bats test_tags=commit-detection,regex
@test "should detect git commit command and reset baseline" {
  # Setup: Create file and initial state
  add_file_to_repo "file.txt" 100
  commit_staged_changes "initial commit"
  local initial_tree=$(git write-tree)

  # Modify file (add 50 lines = 65 points with 1.3× weight)
  append_to_file "file.txt" 50 101 "modified"
  stage_and_capture_tree

  create_session_state "$initial_tree" 400 true 65 "$CURRENT_TREE"

  # Simulate git commit
  commit_staged_changes "fix: update file"
  local new_tree=$(git write-tree)

  # Run PostToolUse hook with git commit command
  local hook_input=$(create_post_tool_use_input "git commit -m 'fix: update file'")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  assert_success

  # Verify session state was reset
  assert_file_exist "$SESSION_STATE_FILE"

  # Check baseline_tree updated to new tree
  assert_json_field_equals "$SESSION_STATE_FILE" ".baseline_tree" "$new_tree"

  # Check accumulated_score reset to 0
  assert_json_field_equals "$SESSION_STATE_FILE" ".accumulated_score" "0"

  # Check stop_triggered reset to false
  assert_json_field_equals "$SESSION_STATE_FILE" ".stop_triggered" "false"

  # Check previous_tree updated to new tree
  assert_json_field_equals "$SESSION_STATE_FILE" ".previous_tree" "$new_tree"
}

# Test 2: Should NOT reset on non-commit git commands
# bats test_tags=commit-detection,negative
@test "should not reset baseline for non-commit git commands" {
  # Setup: Create initial state
  add_file_to_repo "file.txt" 100
  commit_staged_changes "initial"
  local initial_tree=$(git write-tree)

  create_session_state "$initial_tree" 400 true 100

  # Non-commit commands that should NOT trigger reset
  local commands=(
    "git status"
    "git log"
    "git show HEAD"
    "git diff"
    "git branch"
    "git checkout -b feature"
    "git add file.txt"
  )

  for cmd in "${commands[@]}"; do
    # Run hook
    local hook_input=$(create_post_tool_use_input "$cmd")
    run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

    assert_success

    # Verify session state unchanged
    assert_json_field_equals "$SESSION_STATE_FILE" ".baseline_tree" "$initial_tree"
    assert_json_field_equals "$SESSION_STATE_FILE" ".accumulated_score" "100"
    assert_json_field_equals "$SESSION_STATE_FILE" ".stop_triggered" "true"
  done
}

# Test 3: Should handle commit with --message flag variants
# bats test_tags=commit-detection,variations
@test "should detect git commit with various flag formats" {
  # Setup
  add_file_to_repo "file.txt" 50
  commit_staged_changes "initial"
  local initial_tree=$(git write-tree)

  create_session_state "$initial_tree" 400 true 50

  # Different git commit command formats
  local commit_commands=(
    "git commit -m 'message'"
    "git commit --message 'message'"
    "git commit -m'message'"
    "git commit --message='message'"
    "git commit -am 'message'"
    "git commit --all --message 'message'"
    "git -C /some/path commit -m 'message'"
    "git -C \"/path/with spaces\" commit -m 'message'"
  )

  for cmd in "${commit_commands[@]}"; do
    # Simulate commit
    append_to_file "file.txt" 10
    commit_staged_changes "test"
    local new_tree=$(git write-tree)

    # Reset state for next iteration
    create_session_state "$initial_tree" 400 true 50

    # Run hook
    local hook_input=$(create_post_tool_use_input "$cmd")
    run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

    assert_success

    # Verify reset occurred
    assert_json_field_equals "$SESSION_STATE_FILE" ".baseline_tree" "$new_tree"
    assert_json_field_equals "$SESSION_STATE_FILE" ".accumulated_score" "0"
  done
}

# Test 4: Should only reset when session state exists (session-only enforcement)
# bats test_tags=session-enforcement,fail-open
@test "should not reset if no session state exists" {
  # Do NOT create session state
  rm -f "$SESSION_STATE_FILE"

  # Create and commit file
  add_file_to_repo "file.txt" 100
  commit_staged_changes "test commit"

  # Run hook with git commit command
  local hook_input=$(create_post_tool_use_input "git commit -m 'test'")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  # Should succeed (fail-open)
  assert_success

  # Session state should NOT be created
  assert_file_not_exist "$SESSION_STATE_FILE"
}

# Test 5: Should track uncommitted diff after partial commit (Option A)
# bats test_tags=partial-commit,edge-case
@test "should recalculate diff correctly after partial commit" {
  # Setup: Create baseline with initial file
  add_file_to_repo "file1.txt" 100
  commit_staged_changes "initial"
  local baseline_tree=$(git write-tree)

  create_session_state "$baseline_tree" 400 false 0

  # Modify file1 (add 50 lines = 65 points)
  append_to_file "file1.txt" 50 101 "edit"
  git add file1.txt

  # Add new file2 (100 lines = 100 points)
  add_file_to_repo "file2.txt" 100
  git add file2.txt

  # Accumulated score should be: 65 + 100 = 165
  local pre_commit_tree=$(git write-tree)

  # Commit ONLY file1 (partial commit)
  git reset HEAD file2.txt
  # Don't use commit_staged_changes - it does git add . which would re-stage file2
  git commit -m "fix: update file1" -q
  # Get the tree from the actual commit (not index state)
  local tree_after_commit=$(git rev-parse HEAD^{tree})

  # Run PostToolUse hook to trigger reset
  local hook_input=$(create_post_tool_use_input "git commit -m 'fix: update file1'")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  assert_success

  # Verify baseline reset to tree with file1 committed
  assert_json_field_equals "$SESSION_STATE_FILE" ".baseline_tree" "$tree_after_commit"
  assert_json_field_equals "$SESSION_STATE_FILE" ".accumulated_score" "0"

  # Now stage file2 again and check diff calculation
  git add file2.txt
  local tree_with_file2=$(git write-tree)

  # Calculate diff from new baseline to current tree
  # Expected: file2.txt (100 lines) = 100 points
  run git diff-tree --numstat -r "$tree_after_commit" "$tree_with_file2"

  # Verify file2 is still uncommitted (showing in diff)
  assert_output --partial "file2.txt"

  # The uncommitted file2 should show 100 additions
  assert_output --regexp "^100[[:space:]]+"
}

# Test 6: Should clear stop_triggered flag to allow further modifications
# bats test_tags=stop-flag,threshold-enforcement
@test "should clear stop_triggered flag after reset" {
  # Setup with stop_triggered=true (threshold exceeded)
  add_file_to_repo "file.txt" 100
  commit_staged_changes "initial"
  local initial_tree=$(git write-tree)

  create_session_state "$initial_tree" 400 true 450

  # Verify stop_triggered is true before commit
  assert_json_field_equals "$SESSION_STATE_FILE" ".stop_triggered" "true"

  # Commit
  add_file_to_repo "file2.txt" 50
  commit_staged_changes "add file"

  # Run hook
  local hook_input=$(create_post_tool_use_input "git commit -m 'add file'")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  assert_success

  # Verify stop_triggered is now false
  assert_json_field_equals "$SESSION_STATE_FILE" ".stop_triggered" "false"

  # Verify accumulated_score reset
  assert_json_field_equals "$SESSION_STATE_FILE" ".accumulated_score" "0"
}

# Test 7: Should fail open on errors (don't break git workflow)
# bats test_tags=error-handling,fail-open
@test "should exit 0 even if git write-tree fails" {
  # Setup invalid state (corrupt git repo)
  add_file_to_repo "file.txt" 50
  commit_staged_changes "initial"
  local initial_tree=$(git write-tree)

  create_session_state "$initial_tree" 400 false 0

  # Corrupt git objects
  rm -rf .git/objects/*

  # Run hook (should fail to capture tree but exit 0)
  local hook_input=$(create_post_tool_use_input "git commit -m 'test'")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  # Should succeed (fail-open) despite git errors
  assert_success
}

# Test 8: Should handle commit with multiline message
# bats test_tags=commit-detection,multiline
@test "should detect git commit with heredoc message" {
  # Setup
  add_file_to_repo "file.txt" 50
  commit_staged_changes "initial"
  local initial_tree=$(git write-tree)

  create_session_state "$initial_tree" 400 true 50

  # Commit with multiline message (as it appears in hook input)
  add_file_to_repo "file2.txt" 30
  commit_staged_changes "test"
  local new_tree=$(git write-tree)

  # Heredoc format as seen in actual Bash tool calls
  local cmd='git commit -m "$(cat <<'\''EOF'\''
fix: update file

Detailed description here.
EOF
)"'

  # Run hook
  local hook_input=$(create_post_tool_use_input "$cmd")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  assert_success

  # Verify reset occurred
  assert_json_field_equals "$SESSION_STATE_FILE" ".baseline_tree" "$new_tree"
  assert_json_field_equals "$SESSION_STATE_FILE" ".accumulated_score" "0"
}

# Test 9: Should NOT trigger on git commit --amend (edge case consideration)
# bats test_tags=amend,edge-case
@test "should handle git commit --amend" {
  # Setup
  add_file_to_repo "file.txt" 50
  commit_staged_changes "initial"
  local initial_tree=$(git write-tree)

  create_session_state "$initial_tree" 400 true 50

  # Amend commit
  append_to_file "file.txt" 10
  git add file.txt
  git commit --amend --no-edit -q
  local amended_tree=$(git write-tree)

  # Run hook with amend command
  local hook_input=$(create_post_tool_use_input "git commit --amend --no-edit")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  assert_success

  # Should reset (amend is still a commit)
  assert_json_field_equals "$SESSION_STATE_FILE" ".baseline_tree" "$amended_tree"
  assert_json_field_equals "$SESSION_STATE_FILE" ".accumulated_score" "0"
}

# Test 10: Should output feedback message to user and Claude
# bats test_tags=feedback,output
@test "should output success feedback after reset" {
  # Setup
  add_file_to_repo "file.txt" 100
  commit_staged_changes "initial"
  local initial_tree=$(git write-tree)

  create_session_state "$initial_tree" 400 true 100

  # Commit
  add_file_to_repo "file2.txt" 50
  commit_staged_changes "add file"

  # Run hook
  local hook_input=$(create_post_tool_use_input "git commit -m 'add file'")
  run bash "$POST_TOOL_USE_HOOK" <<<"$hook_input"

  assert_success

  # Check for JSON feedback with systemMessage
  assert_output --partial "systemMessage"
  assert_output --partial "Baseline reset"
  assert_output --partial "Threshold budget restored"
}

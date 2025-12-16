# post-tool-use-feedback.bats - Tests for PostToolUse fuel gauge feedback
#
# Purpose: Verify fuel gauge messages reach Claude via stderr + exit 2
# Feature: Progressive threshold warnings without blocking operations

# bats file_tags=unit,posttooluse,feedback,stderr

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'

# Load custom helpers
load '../test_helper/git-test-helpers'

# Source libraries for state setup
PROJECT_ROOT="$BATS_TEST_DIRNAME/../.."
source "$PROJECT_ROOT/bumper-lanes-plugin/hooks/lib/state-manager.sh"
source "$PROJECT_ROOT/bumper-lanes-plugin/hooks/lib/git-state.sh"

# Path to script under test
FEEDBACK_SCRIPT="$PROJECT_ROOT/bumper-lanes-plugin/hooks/entrypoints/post-tool-use-feedback.sh"

# Setup: Create git test repo before each test
setup() {
  setup_git_test_repo

  # Create checkpoint directory
  mkdir -p .git/bumper-checkpoints

  # Create session state with baseline
  SESSION_ID="test-session-$(date +%s)-$$"
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
}

# Teardown: Clean up git repo after each test
teardown() {
  cleanup_git_test_repo
}

# Helper: Create input JSON for hook
make_hook_input() {
  local tool_name="${1:-Write}"
  jq -n \
    --arg session_id "$SESSION_ID" \
    --arg tool_name "$tool_name" \
    '{
      session_id: $session_id,
      tool_name: $tool_name,
      hook_event_name: "PostToolUse"
    }'
}

# Test 1: Below 50% - silent (exit 0)
# bats test_tags=threshold,silent
@test "should exit 0 silently when below 50% threshold" {
  # Add files totaling 150 lines (37.5% of 400)
  add_files_to_repo "file1.txt" 100 "file2.txt" 50

  # Run hook
  run bash -c "echo '$(make_hook_input)' | '$FEEDBACK_SCRIPT'"

  assert_success
  assert_output ""
}

# Test 2: 50-74% - info message
# bats test_tags=threshold,info
@test "should output info to stderr at 50-74% threshold" {
  # Add files totaling 220 lines (55% of 400)
  add_files_to_repo "file1.txt" 120 "file2.txt" 100

  # Run hook (capture stderr separately)
  run bash -c "echo '$(make_hook_input)' | '$FEEDBACK_SCRIPT' 2>&1"

  # Exit 2 means stderr was output
  assert_failure 2
  assert_output --partial "NOTICE"
  assert_output --partial "Wrap up current task"
}

# Test 3: 75-89% - warning message
# bats test_tags=threshold,warning
@test "should output warning to stderr at 75-89% threshold" {
  # Add files totaling 320 lines (80% of 400)
  add_files_to_repo "file1.txt" 160 "file2.txt" 160

  # Run hook
  run bash -c "echo '$(make_hook_input)' | '$FEEDBACK_SCRIPT' 2>&1"

  assert_failure 2
  assert_output --partial "WARNING"
  assert_output --partial "Complete current work"
  assert_output --partial "checkpoint"
}

# Test 4: 90%+ - critical message
# bats test_tags=threshold,critical
@test "should output critical to stderr at 90%+ threshold" {
  # Add files totaling 380 lines (95% of 400)
  add_files_to_repo "file1.txt" 200 "file2.txt" 180

  # Run hook
  run bash -c "echo '$(make_hook_input)' | '$FEEDBACK_SCRIPT' 2>&1"

  assert_failure 2
  assert_output --partial "CRITICAL"
  assert_output --partial "STOP accepting work"
}

# Test 5: Paused - silent regardless of threshold
# bats test_tags=threshold,paused
@test "should exit 0 silently when paused even at high threshold" {
  # Add files well over threshold
  add_files_to_repo "file1.txt" 300 "file2.txt" 200

  # Set paused flag
  set_paused "$SESSION_ID" true

  # Run hook
  run bash -c "echo '$(make_hook_input)' | '$FEEDBACK_SCRIPT'"

  assert_success
  assert_output ""
}

# Test 6: Non-Write/Edit tools are ignored
# bats test_tags=matcher,ignore
@test "should exit 0 for non-Write/Edit tools" {
  # Add files over threshold
  add_files_to_repo "file1.txt" 300

  # Run hook with Bash tool
  run bash -c "echo '$(make_hook_input "Bash")' | '$FEEDBACK_SCRIPT'"

  assert_success
  assert_output ""
}

# Test 7: Edit tool triggers feedback
# bats test_tags=matcher,edit
@test "should trigger feedback for Edit tool" {
  # Add files at 60% threshold
  add_files_to_repo "file1.txt" 240

  # Run hook with Edit tool
  run bash -c "echo '$(make_hook_input "Edit")' | '$FEEDBACK_SCRIPT' 2>&1"

  assert_failure 2
  assert_output --partial "NOTICE"
}

# Test 8: Missing session state - fail open
# bats test_tags=failopen,state
@test "should exit 0 when session state missing" {
  # Add files over threshold
  add_files_to_repo "file1.txt" 300

  # Use non-existent session
  local bad_input
  bad_input=$(jq -n '{
    session_id: "nonexistent-session",
    tool_name: "Write",
    hook_event_name: "PostToolUse"
  }')

  run bash -c "echo '$bad_input' | '$FEEDBACK_SCRIPT'"

  assert_success
}

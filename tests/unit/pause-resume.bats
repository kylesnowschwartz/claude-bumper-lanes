# pause-resume.bats - Tests for pause/resume functionality
#
# Purpose: Verify that pausing enforcement suspends blocking while continuing to track changes
# Feature: /bumper-pause and /bumper-resume slash commands

# bats file_tags=unit,pause,resume,state

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'

# Source state manager directly for unit tests
# Use BATS_TEST_DIRNAME (set by BATS before sourcing) instead of BASH_SOURCE
PROJECT_ROOT="$BATS_TEST_DIRNAME/../.."
source "$PROJECT_ROOT/bumper-lanes-plugin/hooks/lib/state-manager.sh"
source "$PROJECT_ROOT/bumper-lanes-plugin/hooks/lib/git-state.sh"

# Setup: Create git test repo before each test
setup() {
  TEST_REPO="$BATS_TEST_TMPDIR/test-repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"

  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"
  git commit --allow-empty -m "initial" -q

  # Create checkpoint directory
  mkdir -p .git/bumper-checkpoints

  # Create baseline tree
  BASELINE_TREE=$(git write-tree)
  SESSION_ID="test-session-$(date +%s)"
}

# Teardown: Clean up git repo after each test
teardown() {
  if [[ -n "$TEST_REPO" ]] && [[ -d "$TEST_REPO" ]]; then
    cd /
    rm -rf "$TEST_REPO"
  fi
}

# Test 1: set_paused() sets flag to true
# bats test_tags=state,paused
@test "set_paused sets paused flag to true" {
  # Create initial state
  write_session_state "$SESSION_ID" "$BASELINE_TREE"

  # Set paused
  set_paused "$SESSION_ID" true

  # Read state and verify
  local state
  state=$(read_session_state "$SESSION_ID")
  local paused
  paused=$(echo "$state" | jq -r '.paused')

  assert_equal "$paused" "true"
}

# Test 2: set_paused() sets flag to false
# bats test_tags=state,paused
@test "set_paused sets paused flag to false" {
  # Create initial state with paused=true
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_paused "$SESSION_ID" true

  # Unpause
  set_paused "$SESSION_ID" false

  # Read state and verify
  local state
  state=$(read_session_state "$SESSION_ID")
  local paused
  paused=$(echo "$state" | jq -r '.paused')

  assert_equal "$paused" "false"
}

# Test 3: Paused state defaults to false
# bats test_tags=state,paused,default
@test "paused defaults to false for new sessions" {
  # Create initial state
  write_session_state "$SESSION_ID" "$BASELINE_TREE"

  # Read state and verify default
  local state
  state=$(read_session_state "$SESSION_ID")
  local paused
  paused=$(echo "$state" | jq -r '.paused // false')

  assert_equal "$paused" "false"
}

# Test 4: Paused state preserved across other updates
# bats test_tags=state,paused,persistence
@test "paused state preserved when updating other fields" {
  # Create initial state and pause
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_paused "$SESSION_ID" true

  # Update stop_triggered (should not affect paused)
  set_stop_triggered "$SESSION_ID" true

  # Verify paused still true
  local state
  state=$(read_session_state "$SESSION_ID")
  local paused
  paused=$(echo "$state" | jq -r '.paused')

  assert_equal "$paused" "true"
}

# Test 5: Incremental state updates preserve paused flag
# bats test_tags=state,paused,incremental
@test "incremental state updates preserve paused flag" {
  # Create initial state and pause
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_paused "$SESSION_ID" true

  # Update incremental state
  update_incremental_state "$SESSION_ID" "$BASELINE_TREE" 150

  # Verify paused still true
  local state
  state=$(read_session_state "$SESSION_ID")
  local paused
  paused=$(echo "$state" | jq -r '.paused')
  local accumulated_score
  accumulated_score=$(echo "$state" | jq -r '.accumulated_score')

  assert_equal "$paused" "true"
  assert_equal "$accumulated_score" "150"
}

# Test 6: Reset clears paused state (via write_session_state preserving behavior)
# bats test_tags=state,paused,reset
@test "reset baseline should clear paused state" {
  # Create initial state and pause
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_paused "$SESSION_ID" true
  set_stop_triggered "$SESSION_ID" true
  update_incremental_state "$SESSION_ID" "$BASELINE_TREE" 500

  # Simulate reset: update baseline and clear flags
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_stop_triggered "$SESSION_ID" false
  set_paused "$SESSION_ID" false
  update_incremental_state "$SESSION_ID" "$BASELINE_TREE" 0

  # Verify reset state
  local state
  state=$(read_session_state "$SESSION_ID")
  local paused
  paused=$(echo "$state" | jq -r '.paused')
  local stop_triggered
  stop_triggered=$(echo "$state" | jq -r '.stop_triggered')
  local accumulated_score
  accumulated_score=$(echo "$state" | jq -r '.accumulated_score')

  assert_equal "$paused" "false"
  assert_equal "$stop_triggered" "false"
  assert_equal "$accumulated_score" "0"
}

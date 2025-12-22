# view-mode.bats - Tests for diff visualization mode switching
#
# Purpose: Verify view mode setting and persistence for status line diff display
# Feature: /bumper-view slash command with tree|collapsed modes

# bats file_tags=unit,view,mode,state

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'

# Source state manager directly for unit tests
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

# Test 1: set_view_mode sets mode to collapsed
# bats test_tags=state,view_mode
@test "set_view_mode sets view mode to collapsed" {
  write_session_state "$SESSION_ID" "$BASELINE_TREE"

  set_view_mode "$SESSION_ID" collapsed

  local state
  state=$(read_session_state "$SESSION_ID")
  local view_mode
  view_mode=$(echo "$state" | jq -r '.view_mode')

  assert_equal "$view_mode" "collapsed"
}

# Test 2: set_view_mode sets mode to tree
# bats test_tags=state,view_mode
@test "set_view_mode sets view mode to tree" {
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_view_mode "$SESSION_ID" collapsed

  set_view_mode "$SESSION_ID" tree

  local state
  state=$(read_session_state "$SESSION_ID")
  local view_mode
  view_mode=$(echo "$state" | jq -r '.view_mode')

  assert_equal "$view_mode" "tree"
}

# Test 3: get_view_mode returns default "tree" for new sessions
# bats test_tags=state,view_mode,default
@test "view mode defaults to tree for new sessions" {
  write_session_state "$SESSION_ID" "$BASELINE_TREE"

  local view_mode
  view_mode=$(get_view_mode "$SESSION_ID")

  assert_equal "$view_mode" "tree"
}

# Test 4: get_view_mode returns set value
# bats test_tags=state,view_mode
@test "get_view_mode returns previously set value" {
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_view_mode "$SESSION_ID" collapsed

  local view_mode
  view_mode=$(get_view_mode "$SESSION_ID")

  assert_equal "$view_mode" "collapsed"
}

# Test 5: Invalid mode rejected
# bats test_tags=state,view_mode,validation
@test "set_view_mode rejects invalid modes" {
  write_session_state "$SESSION_ID" "$BASELINE_TREE"

  run set_view_mode "$SESSION_ID" "invalid"

  assert_failure
  assert_output --partial "Invalid view mode"
}

# Test 5a: set_view_mode sets mode to smart (sparkline)
# bats test_tags=state,view_mode
@test "set_view_mode sets view mode to smart" {
  write_session_state "$SESSION_ID" "$BASELINE_TREE"

  set_view_mode "$SESSION_ID" smart

  local state
  state=$(read_session_state "$SESSION_ID")
  local view_mode
  view_mode=$(echo "$state" | jq -r '.view_mode')

  assert_equal "$view_mode" "smart"
}

# Test 6: View mode preserved across other updates
# bats test_tags=state,view_mode,persistence
@test "view mode preserved when updating other fields" {
  write_session_state "$SESSION_ID" "$BASELINE_TREE"
  set_view_mode "$SESSION_ID" collapsed

  # Update other state fields
  set_paused "$SESSION_ID" true
  update_incremental_state "$SESSION_ID" "$BASELINE_TREE" 100

  local state
  state=$(read_session_state "$SESSION_ID")
  local view_mode
  view_mode=$(echo "$state" | jq -r '.view_mode')

  assert_equal "$view_mode" "collapsed"
}

# Test 7: get_view_mode returns tree for non-existent session
# bats test_tags=state,view_mode,default
@test "get_view_mode returns tree for non-existent session" {
  local view_mode
  view_mode=$(get_view_mode "non-existent-session")

  assert_equal "$view_mode" "tree"
}

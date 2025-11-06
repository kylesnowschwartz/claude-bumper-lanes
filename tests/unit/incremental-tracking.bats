# incremental-tracking.bats - Tests for delete+recreate bypass bug fix and incremental delta tracking
#
# Purpose: Verify that deleting and recreating a file is correctly tracked
#          via incremental delta calculation (not baseline comparison)
# Migrated from: test-delete-recreate.sh (legacy custom framework)

# bats file_tags=unit,threshold,incremental,tracking

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'
load '../test_helper/bats-file/load'

# Load custom helpers
load '../test_helper/git-test-helpers'
load '../test_helper/threshold-helpers'
load '../test_helper/json-assertions'

# Setup: Create git test repo before each test
setup() {
  setup_git_test_repo
}

# Teardown: Clean up git repo after each test
teardown() {
  cleanup_git_test_repo
}

# Test 1: Delete and recreate file with identical content (THE BUG)
# bats test_tags=bug-fix,delete-recreate
@test "should not count delete-recreate as net zero change" {
  # Step 1: Create baseline with file (450 lines)
  add_file_to_repo "generated-code.txt" 450
  commit_staged_changes "baseline"
  local baseline_tree
  baseline_tree=$(git write-tree)

  # Step 2: Delete file
  rm generated-code.txt
  git add -u
  local tree_after_delete
  tree_after_delete=$(git write-tree)

  # Step 3: Recreate identical file
  add_file_to_repo "generated-code.txt" 450
  stage_and_capture_tree
  local tree_after_recreate="$CURRENT_TREE"

  # Test BEFORE fix (baseline comparison):
  # Baseline has file → Current has identical file → 0 diff (BUG!)
  local old_calculation
  old_calculation=$(calculate_full_threshold "$baseline_tree" "$tree_after_recreate")
  local old_score
  old_score=$(echo "$old_calculation" | jq -r '.weighted_score')

  # Test AFTER fix (incremental tracking):
  # Step A: baseline → delete (deletions ignored = 0)
  local delta1
  delta1=$(calculate_incremental "$baseline_tree" "$tree_after_delete" 0)
  local score_after_delete
  score_after_delete=$(echo "$delta1" | jq -r '.accumulated_score')

  # Step B: delete → recreate (file added = 450)
  local delta2
  delta2=$(calculate_incremental "$tree_after_delete" "$tree_after_recreate" "$score_after_delete")
  local final_score
  final_score=$(echo "$delta2" | jq -r '.accumulated_score')

  # Assertions: Old calculation shows bug (0), new calculation correct (450)
  assert_equal "$old_score" "0"
  assert_equal "$final_score" "450"
}

# Test 2: Delete and recreate with different content
# bats test_tags=delete-recreate,modified
@test "should track cumulative scores across multiple turns" {
  # Baseline with original content (100 lines)
  add_file_to_repo "file.txt" 100 "original"
  commit_staged_changes "baseline"
  local baseline_tree
  baseline_tree=$(git write-tree)

  # Delete file
  rm file.txt
  git add -u
  local tree_after_delete
  tree_after_delete=$(git write-tree)

  # Recreate with different content (150 lines)
  add_file_to_repo "file.txt" 150 "modified"
  stage_and_capture_tree
  local tree_after_recreate="$CURRENT_TREE"

  # Incremental tracking
  local delta1
  delta1=$(calculate_incremental "$baseline_tree" "$tree_after_delete" 0)
  local score_after_delete
  score_after_delete=$(echo "$delta1" | jq -r '.accumulated_score')

  local delta2
  delta2=$(calculate_incremental "$tree_after_delete" "$tree_after_recreate" "$score_after_delete")
  local final_score
  final_score=$(echo "$delta2" | jq -r '.accumulated_score')

  # Expected: 0 (delete ignored) + 150 (recreate as new file) = 150
  assert_equal "$score_after_delete" "0"
  assert_equal "$final_score" "150"
}

# Test 3: Incremental delta tracking across multiple turns
# bats test_tags=incremental,accumulation
@test "should increase accumulated score with each delta" {
  # Turn 1: Add first file (50 lines)
  add_file_to_repo "file1.txt" 50
  stage_and_capture_tree
  local tree_turn1="$CURRENT_TREE"

  local delta1
  delta1=$(calculate_incremental "$BASELINE_TREE" "$tree_turn1" 0)
  local score_turn1
  score_turn1=$(echo "$delta1" | jq -r '.accumulated_score')

  # Turn 2: Add second file (80 lines)
  add_file_to_repo "file2.txt" 80
  stage_and_capture_tree
  local tree_turn2="$CURRENT_TREE"

  local delta2
  delta2=$(calculate_incremental "$tree_turn1" "$tree_turn2" "$score_turn1")
  local score_turn2
  score_turn2=$(echo "$delta2" | jq -r '.accumulated_score')

  # Turn 3: Edit first file (add 30 lines = 30 × 1.3 = 39 points)
  append_to_file "file1.txt" 30 51 "added"
  stage_and_capture_tree
  local tree_turn3="$CURRENT_TREE"

  local delta3
  delta3=$(calculate_incremental "$tree_turn2" "$tree_turn3" "$score_turn2")
  local score_turn3
  score_turn3=$(echo "$delta3" | jq -r '.accumulated_score')

  # Assertions
  assert_equal "$score_turn1" "50"           # 50 new lines
  assert_equal "$score_turn2" "130"          # 50 + 80
  assert_equal "$score_turn3" "169"          # 130 + 39 (30 × 1.3)

  # Verify delta fields show per-turn changes
  local delta3_score
  delta3_score=$(echo "$delta3" | jq -r '.weighted_score')
  assert_equal "$delta3_score" "39"          # Just this turn's delta

  local delta3_accumulated
  delta3_accumulated=$(echo "$delta3" | jq -r '.accumulated_score')
  assert_equal "$delta3_accumulated" "169"   # Total across all turns
}

# weighted-threshold.bats - Unit tests for weighted threshold calculation
#
# Purpose: Validate weighted scoring logic, file scatter penalties, and breakdown formatting
# Migrated from: test-weighted-threshold.sh (legacy custom framework)

# bats file_tags=unit,threshold,weighting,scoring

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

# Test 1: New file only (no weighting)
# bats test_tags=baseline,new-files
@test "should calculate baseline score for new files only" {
  # Add new files (80 + 50 + 70 = 200 lines)
  add_files_to_repo "file1.txt" 80 "file2.txt" 50 "file3.txt" 70

  # Calculate threshold
  local threshold_data
  threshold_data=$(calculate_full_threshold "$BASELINE_TREE" "$CURRENT_TREE")

  # Assertions
  assert_json_field_equals "$threshold_data" ".weighted_score" "200"
  assert_json_field_equals "$threshold_data" ".new_file_additions" "200"
  assert_json_field_equals "$threshold_data" ".files_touched" "3"
  assert_json_field_equals "$threshold_data" ".scatter_penalty" "0"
}

# Test 2: Edited files with 1.3× multiplier
# bats test_tags=weighting,edited-files
@test "should apply 1.3x multiplier to edited files" {
  # Baseline: existing files
  add_files_to_repo "existing1.txt" 10 "existing2.txt" 10
  commit_staged_changes "baseline"
  local baseline_tree
  baseline_tree=$(git write-tree)

  # Modify files (60 + 50 = 110 lines added)
  append_to_file "existing1.txt" 60 11 "new line"
  append_to_file "existing2.txt" 50 11 "new line"
  stage_and_capture_tree

  # Calculate threshold
  local threshold_data
  threshold_data=$(calculate_full_threshold "$baseline_tree" "$CURRENT_TREE")

  # Expected: 110 × 1.3 = 143 points
  assert_json_field_equals "$threshold_data" ".weighted_score" "143"
  assert_json_field_equals "$threshold_data" ".edited_file_additions" "110"
}

# Test 3: File scatter penalty (6-10 files)
# bats test_tags=scatter,penalty
@test "should add 10pts per excess file for 6-10 files touched" {
  # Add 7 new files with 20 lines each = 140 total additions
  add_files_to_repo \
    "file1.txt" 20 "file2.txt" 20 "file3.txt" 20 "file4.txt" 20 \
    "file5.txt" 20 "file6.txt" 20 "file7.txt" 20

  # Calculate threshold
  local threshold_data
  threshold_data=$(calculate_full_threshold "$BASELINE_TREE" "$CURRENT_TREE")

  # Expected: 140 × 1.0 + ((7 - 5) × 10) = 140 + 20 = 160 points
  # Only files above free tier (5) are penalized, not all files
  assert_json_field_equals "$threshold_data" ".files_touched" "7"
  assert_json_field_equals "$threshold_data" ".scatter_penalty" "20"
  assert_json_field_equals "$threshold_data" ".weighted_score" "160"
}

# Test 4: High scatter penalty (11+ files)
# bats test_tags=scatter,penalty
@test "should add 30pts per excess file for 11+ files touched" {
  # Add 12 files with 10 lines each = 120 additions
  add_files_to_repo \
    "file1.txt" 10 "file2.txt" 10 "file3.txt" 10 "file4.txt" 10 \
    "file5.txt" 10 "file6.txt" 10 "file7.txt" 10 "file8.txt" 10 \
    "file9.txt" 10 "file10.txt" 10 "file11.txt" 10 "file12.txt" 10

  # Calculate threshold
  local threshold_data
  threshold_data=$(calculate_full_threshold "$BASELINE_TREE" "$CURRENT_TREE")

  # Expected: 120 × 1.0 + ((12 - 5) × 30) = 120 + 210 = 330 points
  # Only files above free tier (5) are penalized, not all files
  assert_json_field_equals "$threshold_data" ".scatter_penalty" "210"
  assert_json_field_equals "$threshold_data" ".weighted_score" "330"
}

# Test 5: Mixed new and edited files
# bats test_tags=mixed,weighting
@test "should combine new and edited file scores correctly" {
  # Baseline with one existing file
  add_file_to_repo "existing.txt" 10 "original"
  commit_staged_changes "baseline"
  local baseline_tree
  baseline_tree=$(git write-tree)

  # Add to existing (50 lines) + create new file (80 lines)
  append_to_file "existing.txt" 50 11 "added"
  add_file_to_repo "newfile.txt" 80
  stage_and_capture_tree

  # Calculate threshold
  local threshold_data
  threshold_data=$(calculate_full_threshold "$baseline_tree" "$CURRENT_TREE")

  # Expected: (80 × 1.0) + (50 × 1.3) = 80 + 65 = 145 points
  assert_json_field_equals "$threshold_data" ".new_file_additions" "80"
  assert_json_field_equals "$threshold_data" ".edited_file_additions" "50"
  assert_json_field_equals "$threshold_data" ".weighted_score" "145"
}

# Test 6: Format breakdown message
# bats test_tags=formatting,display
@test "should format threshold breakdown with score and percentage" {
  # Create mock threshold data
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 165,
  "new_file_additions": 80,
  "edited_file_additions": 50,
  "files_touched": 3,
  "scatter_penalty": 0
}
EOF
  )

  # Format breakdown
  run format_threshold_breakdown "$threshold_data" 200

  # Should show: "Threshold: 165/200 points (82%)"
  assert_success
  assert_output "Threshold: 165/200 points (82%)"
}

# Test 7: New files in subdirectories should use 1.0× weight (regression test for -r flag bug)
# bats test_tags=regression,subdirectory,new-files
@test "should apply 1.0x weight to new files in subdirectories" {
  # Create nested directory structure with new files
  mkdir -p src/components
  mkdir -p test/nested/deep

  # Add new files in subdirectories (100 + 50 + 30 = 180 lines)
  add_file_to_repo "src/components/Button.tsx" 100
  add_file_to_repo "test/nested/TestFile.txt" 50
  add_file_to_repo "test/nested/deep/DeepFile.md" 30

  # Stage and capture tree
  git add .
  CURRENT_TREE=$(git write-tree)

  # Calculate threshold
  local threshold_data
  threshold_data=$(calculate_full_threshold "$BASELINE_TREE" "$CURRENT_TREE")

  # Expected: 180 × 1.0 = 180 points (no 1.3× multiplier for new files)
  assert_json_field_equals "$threshold_data" ".weighted_score" "180"
  assert_json_field_equals "$threshold_data" ".new_file_additions" "180"
  assert_json_field_equals "$threshold_data" ".edited_file_additions" "0"
  assert_json_field_equals "$threshold_data" ".files_touched" "3"
}

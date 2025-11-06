# test-helpers.bats - Meta-tests for custom test helper libraries
#
# Purpose: Validate that custom helper functions work correctly
# Migrated from: test-lib-functions.sh (legacy custom framework)
#
# Note: These tests validate our custom helpers (git-test-helpers, json-assertions, etc.)
# This is meta-testing - testing the test infrastructure itself

# bats file_tags=unit,test-helpers,meta

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'
load '../test_helper/bats-file/load'

# Load custom helpers
load '../test_helper/git-test-helpers'
load '../test_helper/json-assertions'

# Test 1: setup_git_test_repo creates working repository
# bats test_tags=git-helpers,setup
@test "should create temporary test directory with git repo" {
  # Setup creates TEST_REPO and BASELINE_TREE
  setup_git_test_repo

  # Verify directory exists
  assert_dir_exist "$TEST_REPO"

  # Verify it's a git repo
  run git -C "$TEST_REPO" rev-parse --git-dir
  assert_success
  assert_output ".git"

  # Verify baseline tree exists
  run git -C "$TEST_REPO" cat-file -t "$BASELINE_TREE"
  assert_success
  assert_output "tree"

  # Cleanup
  cleanup_git_test_repo
}

# Test 2: add_file_to_repo creates file with correct line count
# bats test_tags=git-helpers,file-creation
@test "should create file with specified number of lines" {
  setup_git_test_repo

  # Create file with 50 lines
  add_file_to_repo "test.txt" 50

  # Verify file exists and has correct line count
  assert_file_exist "$TEST_REPO/test.txt"
  run bash -c "wc -l < '$TEST_REPO/test.txt' | xargs"
  assert_success
  assert_output "50"

  cleanup_git_test_repo
}

# Test 3: add_files_to_repo creates multiple files and stages them
# bats test_tags=git-helpers,multiple-files
@test "should create and stage multiple files" {
  setup_git_test_repo

  # Create three files
  add_files_to_repo "file1.txt" 10 "file2.txt" 20 "file3.txt" 30

  # Verify all files exist
  assert_file_exist "$TEST_REPO/file1.txt"
  assert_file_exist "$TEST_REPO/file2.txt"
  assert_file_exist "$TEST_REPO/file3.txt"

  # Verify CURRENT_TREE was set
  [ -n "$CURRENT_TREE" ]

  # Verify tree is valid
  run git -C "$TEST_REPO" cat-file -t "$CURRENT_TREE"
  assert_success
  assert_output "tree"

  cleanup_git_test_repo
}

# Test 4: assert_json_field_exists validates field presence
# bats test_tags=json-assertions,validation
@test "should validate JSON field presence" {
  local test_file
  test_file="$BATS_TEST_TMPDIR/test.json"
  echo '{"name": "test", "value": 123}' > "$test_file"

  # Should succeed for existing field
  run assert_json_field_exists "$test_file" ".name"
  assert_success

  # Should fail for missing field
  run assert_json_field_exists "$test_file" ".missing"
  assert_failure
}

# Test 5: assert_json_field_equals validates exact match
# bats test_tags=json-assertions,equality
@test "should validate JSON field exact match" {
  local test_file
  test_file="$BATS_TEST_TMPDIR/test.json"
  echo '{"status": "active"}' > "$test_file"

  # Should succeed for matching value
  run assert_json_field_equals "$test_file" ".status" "active"
  assert_success

  # Should fail for non-matching value
  run assert_json_field_equals "$test_file" ".status" "inactive"
  assert_failure
}

# Test 6: assert_json_field_matches validates regex pattern
# bats test_tags=json-assertions,regex
@test "should validate JSON field regex match" {
  local test_file
  test_file="$BATS_TEST_TMPDIR/test.json"
  echo '{"id": "abc-123-def"}' > "$test_file"

  # Should succeed for matching pattern
  run assert_json_field_matches "$test_file" ".id" "^[a-z0-9-]+$"
  assert_success

  # Should fail for non-matching pattern
  run assert_json_field_matches "$test_file" ".id" "^[0-9]+$"
  assert_failure
}

# Test 7: assert_json_field_type validates correct type
# bats test_tags=json-assertions,types
@test "should validate JSON field type" {
  local test_file
  test_file="$BATS_TEST_TMPDIR/test.json"
  echo '{"enabled": true, "count": 42, "name": "test"}' > "$test_file"

  # Should succeed for correct types
  run assert_json_field_type "$test_file" ".enabled" "boolean"
  assert_success

  run assert_json_field_type "$test_file" ".count" "number"
  assert_success

  run assert_json_field_type "$test_file" ".name" "string"
  assert_success

  # Should fail for incorrect type
  run assert_json_field_type "$test_file" ".enabled" "string"
  assert_failure
}

#!/usr/bin/env bash
set -euo pipefail

# Unit tests for test library functions
# Purpose: Test utility functions without requiring Claude Code

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/test-output.sh"
source "$SCRIPT_DIR/../lib/test-assertions.sh"
source "$SCRIPT_DIR/../lib/test-env.sh"

# Track test results
PASS_COUNT=0
FAIL_COUNT=0

# Test helper
run_test() {
  local test_name="$1"
  local test_func="$2"

  echo ""
  echo "Running: $test_name"

  set +e
  $test_func
  local result=$?
  set -e

  if [[ $result -eq 0 ]]; then
    echo "[PASS] $test_name"
    ((PASS_COUNT++))
  else
    echo "[FAIL] $test_name"
    ((FAIL_COUNT++))
  fi
}

# Test: create_temp_test_dir creates a directory
test_create_temp_dir() {
  local test_dir
  test_dir=$(create_temp_test_dir "unit-test" 2>/dev/null)

  if [[ -d "$test_dir" ]]; then
    rm -rf "$test_dir"
    return 0
  fi
  return 1
}

# Test: find_latest_file finds most recent file
test_find_latest_file() {
  local test_dir
  test_dir=$(mktemp -d "/tmp/test-find.XXXXXX")

  touch "$test_dir/file-1.txt"
  sleep 1
  touch "$test_dir/file-2.txt"

  local latest
  latest=$(find_latest_file "$test_dir" "file-*.txt")

  rm -rf "$test_dir"

  [[ "$latest" == *"file-2.txt" ]]
}

# Test: assert_field_exists with valid JSON
test_assert_field_exists() {
  local test_file
  test_file=$(mktemp)
  echo '{"name": "test", "value": 123}' > "$test_file"

  init_assertions
  assert_field_exists "$test_file" "name" >/dev/null 2>&1
  local result=$?

  rm -f "$test_file"

  [[ $result -eq 0 ]] && assertions_passed
}

# Test: assert_field_equals with matching value
test_assert_field_equals() {
  local test_file
  test_file=$(mktemp)
  echo '{"status": "active"}' > "$test_file"

  init_assertions
  assert_field_equals "$test_file" "status" "active" >/dev/null 2>&1
  local result=$?

  rm -f "$test_file"

  [[ $result -eq 0 ]] && assertions_passed
}

# Test: assert_field_matches with regex pattern
test_assert_field_matches() {
  local test_file
  test_file=$(mktemp)
  echo '{"id": "abc-123-def"}' > "$test_file"

  init_assertions
  assert_field_matches "$test_file" "id" "^[a-z0-9-]+$" >/dev/null 2>&1
  local result=$?

  rm -f "$test_file"

  [[ $result -eq 0 ]] && assertions_passed
}

# Test: assert_field_type with correct type
test_assert_field_type() {
  local test_file
  test_file=$(mktemp)
  echo '{"enabled": true}' > "$test_file"

  init_assertions
  assert_field_type "$test_file" "enabled" "boolean" >/dev/null 2>&1
  local result=$?

  rm -f "$test_file"

  [[ $result -eq 0 ]] && assertions_passed
}

# Test: create_capture_hook generates executable script
test_create_capture_hook() {
  local test_dir
  test_dir=$(mktemp -d "/tmp/test-hook.XXXXXX")

  local hook_file
  hook_file=$(create_capture_hook "$test_dir" "TestHook" "test-output" 2>/dev/null)

  local result=1
  if [[ -f "$hook_file" ]] && [[ -x "$hook_file" ]]; then
    result=0
  fi

  rm -rf "$test_dir"
  return $result
}

# Main test execution
main() {
  test_section "Unit Tests for Test Library Functions"

  run_test "create_temp_test_dir" test_create_temp_dir
  run_test "find_latest_file" test_find_latest_file
  run_test "assert_field_exists" test_assert_field_exists
  run_test "assert_field_equals" test_assert_field_equals
  run_test "assert_field_matches" test_assert_field_matches
  run_test "assert_field_type" test_assert_field_type
  run_test "create_capture_hook" test_create_capture_hook

  test_summary $PASS_COUNT $FAIL_COUNT
}

main

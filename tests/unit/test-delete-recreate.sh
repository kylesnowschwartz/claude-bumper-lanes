#!/usr/bin/env bash
set -euo pipefail

# test-delete-recreate.sh - Test for delete+recreate bypass bug fix
# Purpose: Verify that deleting and recreating a file with identical content
#          is correctly tracked via incremental delta calculation

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/../.."

# Source libraries
source "$PROJECT_ROOT/bumper-lanes-plugin/hooks/lib/threshold-calculator.sh"
source "$PROJECT_ROOT/tests/lib/test-assertions.sh"
source "$PROJECT_ROOT/tests/lib/test-output.sh"

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper: Run a test
run_test() {
  local test_name=$1
  TESTS_RUN=$((TESTS_RUN + 1))

  test_section "$test_name"

  if "$2"; then
    TESTS_PASSED=$((TESTS_PASSED + 1))
    echo "  ✓ PASS"
  else
    TESTS_FAILED=$((TESTS_FAILED + 1))
    echo "  ✗ FAIL"
  fi
  echo ""
}

# Test: Delete and recreate file with identical content (THE BUG)
test_delete_recreate_identical() {
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Step 1: Create baseline with file
  for i in {1..450}; do echo "line $i"; done >generated-code.txt
  git add .
  git commit -m "baseline" -q
  local baseline_tree=$(git write-tree)

  # Step 2: Delete file
  rm generated-code.txt
  git add -u
  local tree_after_delete=$(git write-tree)

  # Step 3: Recreate identical file
  for i in {1..450}; do echo "line $i"; done >generated-code.txt
  git add .
  local tree_after_recreate=$(git write-tree)

  # Test BEFORE fix (baseline comparison):
  # Baseline has file → Current has identical file → 0 diff (BUG!)
  local old_calculation
  old_calculation=$(calculate_weighted_threshold "$baseline_tree" "$tree_after_recreate")
  local old_score
  old_score=$(echo "$old_calculation" | jq -r '.weighted_score')

  # Test AFTER fix (incremental tracking):
  # Step A: baseline → delete (file removed, but deletions ignored = 0)
  local delta1
  delta1=$(calculate_incremental_threshold "$baseline_tree" "$tree_after_delete" 0)
  local score_after_delete
  score_after_delete=$(echo "$delta1" | jq -r '.accumulated_score')

  # Step B: delete → recreate (file added = 450)
  local delta2
  delta2=$(calculate_incremental_threshold "$tree_after_delete" "$tree_after_recreate" "$score_after_delete")
  local final_score
  final_score=$(echo "$delta2" | jq -r '.accumulated_score')

  cd - >/dev/null
  rm -rf "$test_repo"

  # Assertions
  echo "  Old (buggy) calculation: $old_score points"
  echo "  New (fixed) calculation: $final_score points"

  [[ $old_score -eq 0 ]] || {
    echo "  ERROR: Expected old calculation to show 0 (the bug), got $old_score"
    return 1
  }

  [[ $final_score -eq 450 ]] || {
    echo "  ERROR: Expected final score 450, got $final_score"
    return 1
  }

  return 0
}

# Test: Delete and recreate with different content
test_delete_recreate_modified() {
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Baseline with original content
  for i in {1..100}; do echo "original $i"; done >file.txt
  git add .
  git commit -m "baseline" -q
  local baseline_tree=$(git write-tree)

  # Delete file
  rm file.txt
  git add -u
  local tree_after_delete=$(git write-tree)

  # Recreate with different content (200 lines)
  for i in {1..200}; do echo "modified $i"; done >file.txt
  git add .
  local tree_after_recreate=$(git write-tree)

  # Incremental tracking
  local delta1
  delta1=$(calculate_incremental_threshold "$baseline_tree" "$tree_after_delete" 0)
  local score1
  score1=$(echo "$delta1" | jq -r '.accumulated_score')

  local delta2
  delta2=$(calculate_incremental_threshold "$tree_after_delete" "$tree_after_recreate" "$score1")
  local final_score
  final_score=$(echo "$delta2" | jq -r '.accumulated_score')

  cd - >/dev/null
  rm -rf "$test_repo"

  # Expected: 200 points (new file with 200 lines)
  [[ $final_score -eq 200 ]] || {
    echo "  ERROR: Expected 200 points, got $final_score"
    return 1
  }

  return 0
}

# Test: Multiple delete+recreate cycles
test_multiple_cycles() {
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  git commit --allow-empty -m "initial" -q
  local prev_tree=$(git write-tree)
  local accumulated=0

  # Cycle 1: Create 100 lines
  for i in {1..100}; do echo "line $i"; done >file.txt
  git add .
  local tree1=$(git write-tree)
  local delta1
  delta1=$(calculate_incremental_threshold "$prev_tree" "$tree1" "$accumulated")
  accumulated=$(echo "$delta1" | jq -r '.accumulated_score')
  prev_tree=$tree1

  # Cycle 2: Delete
  rm file.txt
  git add -u
  local tree2=$(git write-tree)
  local delta2
  delta2=$(calculate_incremental_threshold "$prev_tree" "$tree2" "$accumulated")
  accumulated=$(echo "$delta2" | jq -r '.accumulated_score')
  prev_tree=$tree2

  # Cycle 3: Recreate with 150 lines
  for i in {1..150}; do echo "line $i"; done >file.txt
  git add .
  local tree3=$(git write-tree)
  local delta3
  delta3=$(calculate_incremental_threshold "$prev_tree" "$tree3" "$accumulated")
  accumulated=$(echo "$delta3" | jq -r '.accumulated_score')

  cd - >/dev/null
  rm -rf "$test_repo"

  # Expected: 100 (create) + 0 (delete) + 150 (recreate) = 250
  [[ $accumulated -eq 250 ]] || {
    echo "  ERROR: Expected 250 points, got $accumulated"
    return 1
  }

  return 0
}

# Run all tests
echo "========================================"
echo " Delete+Recreate Bug Fix Tests"
echo "========================================"
echo ""

run_test "Delete and recreate identical file" test_delete_recreate_identical
run_test "Delete and recreate with modified content" test_delete_recreate_modified
run_test "Multiple delete+recreate cycles" test_multiple_cycles

# Summary
echo "========================================"
echo " Test Summary"
echo "========================================"
echo "Total:  $TESTS_RUN"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo ""

if [[ $TESTS_FAILED -eq 0 ]]; then
  echo "✓ All tests passed!"
  exit 0
else
  echo "✗ Some tests failed"
  exit 1
fi

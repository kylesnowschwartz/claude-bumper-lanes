#!/usr/bin/env bash
set -euo pipefail

# test-weighted-threshold.sh - Unit tests for weighted threshold calculation
# Purpose: Validate weighted scoring logic, file scatter penalties, and breakdown formatting

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

# Test 1: New file only (no weighting)
test_new_files_only() {
  # Setup: Create a test repo with new files
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Baseline: empty commit
  git commit --allow-empty -m "initial" -q
  local baseline_tree=$(git write-tree)

  # Add new files (80 + 50 + 70 = 200 lines)
  {
    for i in {1..80}; do echo "line $i"; done >file1.txt
    for i in {1..50}; do echo "line $i"; done >file2.txt
    for i in {1..70}; do echo "line $i"; done >file3.txt
  }
  git add .
  local current_tree=$(git write-tree)

  # Calculate threshold
  local result
  result=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")

  local weighted_score
  weighted_score=$(echo "$result" | jq -r '.weighted_score')
  local new_file_additions
  new_file_additions=$(echo "$result" | jq -r '.new_file_additions')
  local files_touched
  files_touched=$(echo "$result" | jq -r '.files_touched')
  local scatter_penalty
  scatter_penalty=$(echo "$result" | jq -r '.scatter_penalty')

  # Cleanup
  cd - >/dev/null
  rm -rf "$test_repo"

  # Assertions
  [[ $weighted_score -eq 200 ]] || {
    echo "Expected score 200, got $weighted_score"
    return 1
  }
  [[ $new_file_additions -eq 200 ]] || {
    echo "Expected 200 new additions, got $new_file_additions"
    return 1
  }
  [[ $files_touched -eq 3 ]] || {
    echo "Expected 3 files, got $files_touched"
    return 1
  }
  [[ $scatter_penalty -eq 0 ]] || {
    echo "Expected 0 scatter penalty, got $scatter_penalty"
    return 1
  }

  return 0
}

# Test 2: Edited files with 1.3× multiplier
test_edited_files_weighting() {
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Baseline: existing files
  for i in {1..10}; do echo "original line $i"; done >existing1.txt
  for i in {1..10}; do echo "original line $i"; done >existing2.txt
  git add .
  git commit -m "baseline" -q
  local baseline_tree=$(git write-tree)

  # Modify files (60 + 50 = 110 lines added)
  for i in {11..70}; do echo "new line $i"; done >>existing1.txt
  for i in {11..60}; do echo "new line $i"; done >>existing2.txt
  git add .
  local current_tree=$(git write-tree)

  # Calculate threshold
  local result
  result=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")

  local weighted_score
  weighted_score=$(echo "$result" | jq -r '.weighted_score')
  local edited_file_additions
  edited_file_additions=$(echo "$result" | jq -r '.edited_file_additions')

  cd - >/dev/null
  rm -rf "$test_repo"

  # Expected: 110 × 1.3 = 143 points
  [[ $weighted_score -eq 143 ]] || {
    echo "Expected score 143, got $weighted_score"
    return 1
  }
  [[ $edited_file_additions -eq 110 ]] || {
    echo "Expected 110 edits, got $edited_file_additions"
    return 1
  }

  return 0
}

# Test 3: File scatter penalty (6-10 files)
test_scatter_penalty_medium() {
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Baseline
  git commit --allow-empty -m "initial" -q
  local baseline_tree=$(git write-tree)

  # Add 7 new files with 20 lines each = 140 total additions
  for file_num in {1..7}; do
    for i in {1..20}; do echo "line $i"; done >"file${file_num}.txt"
  done
  git add .
  local current_tree=$(git write-tree)

  local result
  result=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")

  local weighted_score
  weighted_score=$(echo "$result" | jq -r '.weighted_score')
  local files_touched
  files_touched=$(echo "$result" | jq -r '.files_touched')
  local scatter_penalty
  scatter_penalty=$(echo "$result" | jq -r '.scatter_penalty')

  cd - >/dev/null
  rm -rf "$test_repo"

  # Expected: 140 × 1.0 + (7 × 10) = 140 + 70 = 210 points
  [[ $files_touched -eq 7 ]] || {
    echo "Expected 7 files, got $files_touched"
    return 1
  }
  [[ $scatter_penalty -eq 70 ]] || {
    echo "Expected 70 penalty, got $scatter_penalty"
    return 1
  }
  [[ $weighted_score -eq 210 ]] || {
    echo "Expected score 210, got $weighted_score"
    return 1
  }

  return 0
}

# Test 4: High scatter penalty (11+ files)
test_scatter_penalty_high() {
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  git commit --allow-empty -m "initial" -q
  local baseline_tree=$(git write-tree)

  # Add 12 files with 10 lines each = 120 additions
  for file_num in {1..12}; do
    for i in {1..10}; do echo "line $i"; done >"file${file_num}.txt"
  done
  git add .
  local current_tree=$(git write-tree)

  local result
  result=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")

  local scatter_penalty
  scatter_penalty=$(echo "$result" | jq -r '.scatter_penalty')
  local weighted_score
  weighted_score=$(echo "$result" | jq -r '.weighted_score')

  cd - >/dev/null
  rm -rf "$test_repo"

  # Expected: 120 × 1.0 + (12 × 30) = 120 + 360 = 480 points
  [[ $scatter_penalty -eq 360 ]] || {
    echo "Expected 360 penalty, got $scatter_penalty"
    return 1
  }
  [[ $weighted_score -eq 480 ]] || {
    echo "Expected score 480, got $weighted_score"
    return 1
  }

  return 0
}

# Test 5: Mixed new and edited files
test_mixed_changes() {
  local test_repo=$(mktemp -d)
  cd "$test_repo"
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Baseline with one existing file
  for i in {1..10}; do echo "original $i"; done >existing.txt
  git add .
  git commit -m "baseline" -q
  local baseline_tree=$(git write-tree)

  # Add to existing (50 lines) + create new file (80 lines)
  for i in {11..60}; do echo "added $i"; done >>existing.txt
  for i in {1..80}; do echo "line $i"; done >newfile.txt
  git add .
  local current_tree=$(git write-tree)

  local result
  result=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")

  local weighted_score
  weighted_score=$(echo "$result" | jq -r '.weighted_score')
  local new_file_additions
  new_file_additions=$(echo "$result" | jq -r '.new_file_additions')
  local edited_file_additions
  edited_file_additions=$(echo "$result" | jq -r '.edited_file_additions')

  cd - >/dev/null
  rm -rf "$test_repo"

  # Expected: (80 × 1.0) + (50 × 1.3) = 80 + 65 = 145 points
  [[ $new_file_additions -eq 80 ]] || {
    echo "Expected 80 new, got $new_file_additions"
    return 1
  }
  [[ $edited_file_additions -eq 50 ]] || {
    echo "Expected 50 edits, got $edited_file_additions"
    return 1
  }
  [[ $weighted_score -eq 145 ]] || {
    echo "Expected score 145, got $weighted_score"
    return 1
  }

  return 0
}

# Test 6: Format breakdown message
test_format_breakdown() {
  # Create mock threshold data
  local threshold_data='{
    "weighted_score": 165,
    "new_file_additions": 80,
    "edited_file_additions": 50,
    "files_touched": 3,
    "scatter_penalty": 0
  }'

  local output
  output=$(format_threshold_breakdown "$threshold_data" 200)

  # Check output contains key information
  echo "$output" | grep -q "165/200" || {
    echo "Missing score ratio"
    return 1
  }
  echo "$output" | grep -q "New code: 80 lines" || {
    echo "Missing new code"
    return 1
  }
  echo "$output" | grep -q "Edited code: 50 lines" || {
    echo "Missing edited code"
    return 1
  }
  echo "$output" | grep -q "3 files" || {
    echo "Missing file count"
    return 1
  }

  return 0
}

# Run all tests
echo "========================================"
echo " Weighted Threshold Calculator Tests"
echo "========================================"
echo ""

run_test "New files only (1.0× weight)" test_new_files_only
run_test "Edited files (1.3× weight)" test_edited_files_weighting
run_test "File scatter penalty (6-10 files)" test_scatter_penalty_medium
run_test "High scatter penalty (11+ files)" test_scatter_penalty_high
run_test "Mixed new and edited files" test_mixed_changes
run_test "Format breakdown message" test_format_breakdown

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

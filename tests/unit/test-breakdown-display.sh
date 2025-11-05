#!/usr/bin/env bash
set -euo pipefail

# test-breakdown-display.sh - TDD test for format_threshold_breakdown fix
# Purpose: Validate that breakdown formatting works correctly with constants

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../../bumper-lanes-plugin/hooks/lib/threshold-calculator.sh"

# ANSI colors for test output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Test counter
tests_run=0
tests_passed=0

# Test helper
run_test() {
  local test_name=$1
  tests_run=$((tests_run + 1))

  echo "Running: $test_name"

  if "$test_name"; then
    tests_passed=$((tests_passed + 1))
    echo -e "${GREEN}✓ PASS${NC}: $test_name"
  else
    echo -e "${RED}✗ FAIL${NC}: $test_name"
  fi
  echo ""
}

# Test 1: Basic score display
test_new_file_breakdown() {
  local threshold_data
  threshold_data=$(
    cat <<'EOF'
{
  "weighted_score": 100,
  "new_file_additions": 100,
  "edited_file_additions": 0,
  "files_touched": 1,
  "scatter_penalty": 0
}
EOF
  )

  local output
  output=$(format_threshold_breakdown "$threshold_data" 200)

  # Should be single line with score and percentage
  echo "$output" | grep -q "^Threshold: 100/200 points (50%)$" || {
    echo "ERROR: Expected 'Threshold: 100/200 points (50%)'"
    echo "Output: $output"
    return 1
  }

  return 0
}

# Test 2: Score display with higher threshold
test_edited_file_breakdown() {
  local threshold_data
  threshold_data=$(
    cat <<'EOF'
{
  "weighted_score": 130,
  "new_file_additions": 0,
  "edited_file_additions": 100,
  "files_touched": 1,
  "scatter_penalty": 0
}
EOF
  )

  local output
  output=$(format_threshold_breakdown "$threshold_data" 200)

  # Should show correct score
  echo "$output" | grep -q "^Threshold: 130/200 points (65%)$" || {
    echo "ERROR: Expected 'Threshold: 130/200 points (65%)'"
    echo "Output: $output"
    return 1
  }

  return 0
}

# Test 3: Different threshold limit
test_mixed_breakdown() {
  local threshold_data
  threshold_data=$(
    cat <<'EOF'
{
  "weighted_score": 230,
  "new_file_additions": 100,
  "edited_file_additions": 100,
  "files_touched": 2,
  "scatter_penalty": 0
}
EOF
  )

  local output
  output=$(format_threshold_breakdown "$threshold_data" 400)

  # Should show correct percentage calculation
  echo "$output" | grep -q "^Threshold: 230/400 points (57%)$" || {
    echo "ERROR: Expected 'Threshold: 230/400 points (57%)'"
    echo "Output: $output"
    return 1
  }

  return 0
}

# Test 4: Score with scatter penalty
test_scatter_penalty_low() {
  local threshold_data
  threshold_data=$(
    cat <<'EOF'
{
  "weighted_score": 160,
  "new_file_additions": 100,
  "edited_file_additions": 0,
  "files_touched": 6,
  "scatter_penalty": 60
}
EOF
  )

  local output
  output=$(format_threshold_breakdown "$threshold_data" 400)

  # Simplified output - just score
  echo "$output" | grep -q "^Threshold: 160/400 points (40%)$" || {
    echo "ERROR: Expected 'Threshold: 160/400 points (40%)'"
    echo "Output: $output"
    return 1
  }

  return 0
}

# Test 5: Lower score scenario
test_no_scatter_warning() {
  local threshold_data
  threshold_data=$(
    cat <<'EOF'
{
  "weighted_score": 100,
  "new_file_additions": 100,
  "edited_file_additions": 0,
  "files_touched": 3,
  "scatter_penalty": 0
}
EOF
  )

  local output
  output=$(format_threshold_breakdown "$threshold_data" 400)

  # Simple score output
  echo "$output" | grep -q "^Threshold: 100/400 points (25%)$" || {
    echo "ERROR: Expected 'Threshold: 100/400 points (25%)'"
    echo "Output: $output"
    return 1
  }

  return 0
}

# Test 6: Threshold exceeded (>100%)
test_threshold_exceeded() {
  local threshold_data
  threshold_data=$(
    cat <<'EOF'
{
  "weighted_score": 501,
  "new_file_additions": 500,
  "edited_file_additions": 0,
  "files_touched": 1,
  "scatter_penalty": 0
}
EOF
  )

  local output
  output=$(format_threshold_breakdown "$threshold_data" 400)

  # Should show >100% percentage
  echo "$output" | grep -q "^Threshold: 501/400 points (125%)$" || {
    echo "ERROR: Expected 'Threshold: 501/400 points (125%)'"
    echo "Output: $output"
    return 1
  }

  return 0
}

# Test 7: Incremental tracking with accumulated score (FIXED)
# After simplification, format shows only score/percentage - no misleading breakdowns
test_incremental_delta_display() {
  # Simulate what stop.sh sees after incremental tracking
  # Delta this turn: 50 points
  # Accumulated total: 586 points
  local threshold_data
  threshold_data=$(
    cat <<'EOF'
{
  "weighted_score": 50,
  "new_file_additions": 50,
  "edited_file_additions": 0,
  "files_touched": 1,
  "scatter_penalty": 0,
  "accumulated_score": 586,
  "delta_score": 50
}
EOF
  )

  # Transform like stop.sh does
  local threshold_data_for_display
  threshold_data_for_display=$(echo "$threshold_data" | jq '.weighted_score = .accumulated_score')

  local output
  output=$(format_threshold_breakdown "$threshold_data_for_display" 400)

  # Should show correct accumulated total - clean and unambiguous
  echo "$output" | grep -q "^Threshold: 586/400 points (146%)$" || {
    echo "ERROR: Expected 'Threshold: 586/400 points (146%)'"
    echo "Output: $output"
    return 1
  }

  return 0
}

# Run all tests
echo "========================================"
echo "Testing format_threshold_breakdown()"
echo "========================================"
echo ""

run_test test_new_file_breakdown
run_test test_edited_file_breakdown
run_test test_mixed_breakdown
run_test test_scatter_penalty_low
run_test test_no_scatter_warning
run_test test_threshold_exceeded
run_test test_incremental_delta_display

# Summary
echo "========================================"
echo "Results: $tests_passed/$tests_run tests passed"
echo "========================================"

if [[ $tests_passed -eq $tests_run ]]; then
  echo -e "${GREEN}All tests passed!${NC}"
  exit 0
else
  echo -e "${RED}Some tests failed${NC}"
  exit 1
fi

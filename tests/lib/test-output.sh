#!/usr/bin/env bash
# test-output.sh - Standardized test output utilities
# Purpose: Provide consistent test output formatting without emojis (CDP-007)

# Print a test section header
# Usage: test_section "Section Name"
test_section() {
  local section_name="$1"
  echo ""
  echo "=== $section_name ==="
  echo ""
}

# Print a success message
# Usage: test_pass "Message"
test_pass() {
  local message="$1"
  echo "[PASS] $message"
}

# Print a failure message
# Usage: test_fail "Message"
test_fail() {
  local message="$1"
  echo "[FAIL] $message"
}

# Print an info message
# Usage: test_info "Message"
test_info() {
  local message="$1"
  echo "[INFO] $message"
}

# Print a test summary
# Usage: test_summary pass_count fail_count
test_summary() {
  local pass_count="$1"
  local fail_count="$2"
  local total=$((pass_count + fail_count))

  echo ""
  echo "=========================================="
  echo "Test Summary"
  echo "=========================================="
  echo "Total:  $total"
  echo "Passed: $pass_count"
  echo "Failed: $fail_count"
  echo "=========================================="

  if [[ $fail_count -eq 0 ]]; then
    echo "Result: ALL TESTS PASSED"
    return 0
  else
    echo "Result: TESTS FAILED"
    return 1
  fi
}

#!/usr/bin/env bash
# hook-test-utils.sh - Hook-specific testing utilities
# Purpose: Provide reusable patterns for testing Claude Code hooks

# Source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-output.sh"
source "$SCRIPT_DIR/test-assertions.sh"
source "$SCRIPT_DIR/test-env.sh"

# Validate a hook capture with expected fields and values
# Usage: validate_hook_capture hook_name json_file required_fields expected_hook_event_name
# Example: validate_hook_capture "SessionStart" "$json_file" "session_id transcript_path cwd" "SessionStart"
validate_hook_capture() {
  local hook_name="$1"
  local json_file="$2"
  local required_fields="$3"
  local expected_hook_event_name="$4"

  test_section "Validating $hook_name hook schema"

  # Check if file exists
  if [[ ! -f "$json_file" ]]; then
    test_fail "No $hook_name capture found"
    return 1
  fi

  init_assertions

  # Check all required fields exist
  assert_fields_exist "$json_file" "$required_fields"

  # Validate hook_event_name
  assert_field_equals "$json_file" "hook_event_name" "$expected_hook_event_name"

  # Report results
  if assertions_passed; then
    test_pass "$hook_name schema validation PASSED"
    return 0
  else
    local error_count
    error_count=$(get_assertion_errors)
    test_fail "$hook_name schema validation FAILED ($error_count errors)"
    return 1
  fi
}

# Validate SessionStart hook with UUID check
# Usage: validate_session_start_capture json_file
validate_session_start_capture() {
  local json_file="$1"

  test_section "Validating SessionStart hook schema"

  if [[ ! -f "$json_file" ]]; then
    test_fail "No SessionStart capture found"
    return 1
  fi

  init_assertions

  # Check required fields
  assert_fields_exist "$json_file" "session_id transcript_path cwd hook_event_name source"

  # Validate hook_event_name
  assert_field_equals "$json_file" "hook_event_name" "SessionStart"

  # Validate session_id is UUID format
  assert_field_matches "$json_file" "session_id" "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

  # Report results
  if assertions_passed; then
    test_pass "SessionStart schema validation PASSED"
    return 0
  else
    local error_count
    error_count=$(get_assertion_errors)
    test_fail "SessionStart schema validation FAILED ($error_count errors)"
    return 1
  fi
}

# Validate Stop hook with boolean check
# Usage: validate_stop_capture json_file
validate_stop_capture() {
  local json_file="$1"

  test_section "Validating Stop hook schema"

  if [[ ! -f "$json_file" ]]; then
    test_fail "No Stop capture found"
    return 1
  fi

  init_assertions

  # Check required fields
  assert_fields_exist "$json_file" "session_id transcript_path cwd permission_mode hook_event_name stop_hook_active"

  # Validate hook_event_name
  assert_field_equals "$json_file" "hook_event_name" "Stop"

  # Validate stop_hook_active is boolean
  assert_field_type "$json_file" "stop_hook_active" "boolean"

  # Report results
  if assertions_passed; then
    test_pass "Stop schema validation PASSED"
    return 0
  else
    local error_count
    error_count=$(get_assertion_errors)
    test_fail "Stop schema validation FAILED ($error_count errors)"
    return 1
  fi
}

# Check environment variables in captured env file
# Usage: check_env_vars env_file "VAR1 VAR2 VAR3"
check_env_vars() {
  local env_file="$1"
  local required_vars="$2"

  test_section "Validating environment variables"

  if [[ ! -f "$env_file" ]]; then
    test_fail "No environment capture found"
    return 1
  fi

  init_assertions

  # Check each required variable
  for var in $required_vars; do
    if grep -q "^${var}=" "$env_file"; then
      test_pass "Variable present: $var"
    else
      test_fail "Missing environment variable: $var"
      ((_ASSERTION_ERRORS++))
    fi
  done

  # Report results
  if assertions_passed; then
    test_pass "Environment validation PASSED"
    return 0
  else
    local error_count
    error_count=$(get_assertion_errors)
    test_fail "Environment validation FAILED ($error_count errors)"
    return 1
  fi
}

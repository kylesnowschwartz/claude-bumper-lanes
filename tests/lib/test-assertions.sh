#!/usr/bin/env bash
# test-assertions.sh - JSON validation and assertion utilities
# Purpose: Provide reusable assertion functions for testing

# Source test output for reporting
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-output.sh"

# Global error counter for tracking assertion failures
_ASSERTION_ERRORS=0

# Initialize assertion tracking for a test
# Usage: init_assertions
init_assertions() {
  _ASSERTION_ERRORS=0
}

# Get current assertion error count
# Usage: get_assertion_errors
get_assertion_errors() {
  echo "$_ASSERTION_ERRORS"
}

# Assert that a JSON field exists
# Usage: assert_field_exists json_file field_name
# Returns: 0 if field exists, increments error counter otherwise
assert_field_exists() {
  local json_file="$1"
  local field_name="$2"

  if jq -e "has(\"$field_name\")" "$json_file" >/dev/null 2>&1; then
    test_pass "Field present: .$field_name"
    return 0
  else
    test_fail "Missing required field: .$field_name"
    ((_ASSERTION_ERRORS++))
    return 1
  fi
}

# Assert that a JSON field equals an expected value
# Usage: assert_field_equals json_file field_name expected_value
assert_field_equals() {
  local json_file="$1"
  local field_name="$2"
  local expected_value="$3"

  local actual_value
  actual_value=$(jq -r ".$field_name" "$json_file" 2>/dev/null)

  if [[ "$actual_value" == "$expected_value" ]]; then
    test_pass "Field .$field_name = '$expected_value'"
    return 0
  else
    test_fail "Field .$field_name = '$actual_value' (expected: '$expected_value')"
    ((_ASSERTION_ERRORS++))
    return 1
  fi
}

# Assert that a JSON field matches a regex pattern
# Usage: assert_field_matches json_file field_name regex_pattern
assert_field_matches() {
  local json_file="$1"
  local field_name="$2"
  local pattern="$3"

  local actual_value
  actual_value=$(jq -r ".$field_name" "$json_file" 2>/dev/null)

  if [[ "$actual_value" =~ $pattern ]]; then
    test_pass "Field .$field_name matches pattern '$pattern'"
    return 0
  else
    test_fail "Field .$field_name = '$actual_value' does not match pattern '$pattern'"
    ((_ASSERTION_ERRORS++))
    return 1
  fi
}

# Assert that a JSON field is of a specific type
# Usage: assert_field_type json_file field_name type
# Types: string, number, boolean, array, object, null
assert_field_type() {
  local json_file="$1"
  local field_name="$2"
  local expected_type="$3"

  local actual_type
  actual_type=$(jq -r ".$field_name | type" "$json_file" 2>/dev/null)

  if [[ "$actual_type" == "$expected_type" ]]; then
    test_pass "Field .$field_name is type '$expected_type'"
    return 0
  else
    test_fail "Field .$field_name is type '$actual_type' (expected: '$expected_type')"
    ((_ASSERTION_ERRORS++))
    return 1
  fi
}

# Assert that multiple fields exist in JSON
# Usage: assert_fields_exist json_file "field1 field2 field3"
assert_fields_exist() {
  local json_file="$1"
  local fields="$2"

  for field in $fields; do
    assert_field_exists "$json_file" "$field"
  done
}

# Check if assertions passed (no errors)
# Usage: assertions_passed
# Returns: 0 if no errors, 1 if errors occurred
assertions_passed() {
  if [[ $_ASSERTION_ERRORS -eq 0 ]]; then
    return 0
  else
    return 1
  fi
}

# json-assertions.bash - JSON validation assertions for Bats
#
# Purpose: Provide reusable JSON assertion functions compatible with Bats
# Migrated from: tests/lib/test-assertions.sh (legacy custom framework)
#
# These functions work with both JSON files and JSON strings.
# They integrate with bats-assert for consistent error formatting.
#
# Note: Load bats-support before loading this file

# assert_json_field_exists() - Assert that a JSON field exists
#
# Args:
#   $1 - JSON source (file path or JSON string)
#   $2 - field selector (jq format, e.g., ".status", ".nested.field")
#
# Usage:
#   assert_json_field_exists "$json_file" ".status"
#   assert_json_field_exists "$json_string" ".result.code"
assert_json_field_exists() {
  local json_source="$1"
  local field_selector="$2"

  local json_input
  if [[ -f "$json_source" ]]; then
    json_input=$(cat "$json_source")
  else
    json_input="$json_source"
  fi

  # Check if the field exists using jq path expression
  if echo "$json_input" | jq -e "$field_selector" >/dev/null 2>&1; then
    return 0
  else
    batslib_print_kv_single_or_multi 8 \
      'field' "$field_selector" \
      'error' "Field does not exist in JSON" >&2
    return 1
  fi
}

# assert_json_field_equals() - Assert that a JSON field equals expected value
#
# Args:
#   $1 - JSON source (file path or JSON string)
#   $2 - field selector (jq format)
#   $3 - expected value
#
# Usage:
#   assert_json_field_equals "$json_file" ".status" "active"
#   assert_json_field_equals "$json_string" ".code" "200"
assert_json_field_equals() {
  local json_source="$1"
  local field_selector="$2"
  local expected_value="$3"

  local json_input
  if [[ -f "$json_source" ]]; then
    json_input=$(cat "$json_source")
  else
    json_input="$json_source"
  fi

  local actual_value
  actual_value=$(echo "$json_input" | jq -r "$field_selector" 2>/dev/null)

  if [[ "$actual_value" == "$expected_value" ]]; then
    return 0
  else
    batslib_print_kv_single_or_multi 8 \
      'field' "$field_selector" \
      'expected' "$expected_value" \
      'actual' "$actual_value" >&2
    return 1
  fi
}

# assert_json_field_matches() - Assert that a JSON field matches regex pattern
#
# Args:
#   $1 - JSON source (file path or JSON string)
#   $2 - field selector (jq format)
#   $3 - regex pattern
#
# Usage:
#   assert_json_field_matches "$json_file" ".id" "^[a-z0-9-]+$"
#   assert_json_field_matches "$json_string" ".version" "^[0-9]+\.[0-9]+\.[0-9]+$"
assert_json_field_matches() {
  local json_source="$1"
  local field_selector="$2"
  local pattern="$3"

  local json_input
  if [[ -f "$json_source" ]]; then
    json_input=$(cat "$json_source")
  else
    json_input="$json_source"
  fi

  local actual_value
  actual_value=$(echo "$json_input" | jq -r "$field_selector" 2>/dev/null)

  if [[ "$actual_value" =~ $pattern ]]; then
    return 0
  else
    batslib_print_kv_single_or_multi 8 \
      'field' "$field_selector" \
      'pattern' "$pattern" \
      'actual' "$actual_value" >&2
    return 1
  fi
}

# assert_json_field_type() - Assert that a JSON field is of specific type
#
# Args:
#   $1 - JSON source (file path or JSON string)
#   $2 - field selector (jq format)
#   $3 - expected type (string, number, boolean, array, object, null)
#
# Usage:
#   assert_json_field_type "$json_file" ".enabled" "boolean"
#   assert_json_field_type "$json_string" ".count" "number"
assert_json_field_type() {
  local json_source="$1"
  local field_selector="$2"
  local expected_type="$3"

  local json_input
  if [[ -f "$json_source" ]]; then
    json_input=$(cat "$json_source")
  else
    json_input="$json_source"
  fi

  local actual_type
  actual_type=$(echo "$json_input" | jq -r "$field_selector | type" 2>/dev/null)

  if [[ "$actual_type" == "$expected_type" ]]; then
    return 0
  else
    batslib_print_kv_single_or_multi 8 \
      'field' "$field_selector" \
      'expected type' "$expected_type" \
      'actual type' "$actual_type" >&2
    return 1
  fi
}

# assert_json_fields_exist() - Assert that multiple fields exist
#
# Args:
#   $1 - JSON source (file path or JSON string)
#   $2+ - field selectors (jq format)
#
# Usage:
#   assert_json_fields_exist "$json_file" ".status" ".code" ".message"
assert_json_fields_exist() {
  local json_source="$1"
  shift

  for field_selector in "$@"; do
    assert_json_field_exists "$json_source" "$field_selector" || return 1
  done
}

# assert_valid_json() - Assert that input is valid JSON
#
# Args:
#   $1 - JSON source (file path or JSON string)
#
# Usage:
#   assert_valid_json "$json_file"
#   assert_valid_json "$json_string"
assert_valid_json() {
  local json_source="$1"

  local json_input
  if [[ -f "$json_source" ]]; then
    json_input=$(cat "$json_source")
  else
    json_input="$json_source"
  fi

  if echo "$json_input" | jq empty >/dev/null 2>&1; then
    return 0
  else
    batslib_print_kv_single_or_multi 8 \
      'error' "Invalid JSON" \
      'source' "${json_source:0:100}..." >&2
    return 1
  fi
}

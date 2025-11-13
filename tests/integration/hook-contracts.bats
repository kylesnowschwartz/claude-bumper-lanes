# hook-contracts.bats - Hook Contract Validation Test Suite
#
# Purpose: Validate hook input JSON schemas against expected contracts
# Uses fixture files instead of running Claude Code
# Migrated from: validate-hook-contracts.sh (legacy custom framework)

# bats file_tags=integration,hooks,contracts

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'
load '../test_helper/bats-file/load'

# Load custom helpers
load '../test_helper/json-assertions'

# Fixture paths
FIXTURES_DIR="$BATS_TEST_DIRNAME/../fixtures"
SESSION_START_FIXTURE="$FIXTURES_DIR/session-start.json"
SESSION_END_FIXTURE="$FIXTURES_DIR/session-end.json"
STOP_FIXTURE="$FIXTURES_DIR/stop.json"
POST_TOOL_USE_FIXTURE="$FIXTURES_DIR/post-tool-use-bash-commit.json"
ENV_FIXTURE="$FIXTURES_DIR/test-env.txt"

# Test 1: Validate SessionStart hook JSON schema
# bats test_tags=session-start,schema
@test "should validate SessionStart hook JSON schema" {
  # Verify fixture exists
  assert_file_exist "$SESSION_START_FIXTURE"

  # Validate JSON is well-formed
  run jq empty "$SESSION_START_FIXTURE"
  assert_success

  # Check required fields exist
  assert_json_fields_exist "$SESSION_START_FIXTURE" \
    ".session_id" \
    ".transcript_path" \
    ".cwd" \
    ".hook_event_name" \
    ".source"

  # Validate hook_event_name value
  assert_json_field_equals "$SESSION_START_FIXTURE" ".hook_event_name" "SessionStart"

  # Validate session_id is UUID format
  assert_json_field_matches "$SESSION_START_FIXTURE" ".session_id" \
    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

  # Validate field types
  assert_json_field_type "$SESSION_START_FIXTURE" ".session_id" "string"
  assert_json_field_type "$SESSION_START_FIXTURE" ".transcript_path" "string"
  assert_json_field_type "$SESSION_START_FIXTURE" ".cwd" "string"
}

# Test 2: Validate Stop hook JSON schema
# bats test_tags=stop,schema
@test "should validate Stop hook JSON schema" {
  # Verify fixture exists
  assert_file_exist "$STOP_FIXTURE"

  # Validate JSON is well-formed
  run jq empty "$STOP_FIXTURE"
  assert_success

  # Check required fields exist
  assert_json_fields_exist "$STOP_FIXTURE" \
    ".session_id" \
    ".transcript_path" \
    ".cwd" \
    ".hook_event_name"

  # Validate hook_event_name value
  assert_json_field_equals "$STOP_FIXTURE" ".hook_event_name" "Stop"

  # Validate session_id is UUID format
  assert_json_field_matches "$STOP_FIXTURE" ".session_id" \
    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

  # Validate field types
  assert_json_field_type "$STOP_FIXTURE" ".session_id" "string"
  assert_json_field_type "$STOP_FIXTURE" ".transcript_path" "string"
  assert_json_field_type "$STOP_FIXTURE" ".cwd" "string"
}

# Test 3: Validate SessionEnd hook JSON schema
# bats test_tags=session-end,schema
@test "should validate SessionEnd hook JSON schema" {
  # Verify fixture exists
  assert_file_exist "$SESSION_END_FIXTURE"

  # Validate JSON is well-formed
  run jq empty "$SESSION_END_FIXTURE"
  assert_success

  # Check required fields exist
  assert_json_fields_exist "$SESSION_END_FIXTURE" \
    ".session_id" \
    ".transcript_path" \
    ".cwd" \
    ".hook_event_name" \
    ".reason"

  # Validate hook_event_name value
  assert_json_field_equals "$SESSION_END_FIXTURE" ".hook_event_name" "SessionEnd"

  # Validate session_id is UUID format
  assert_json_field_matches "$SESSION_END_FIXTURE" ".session_id" \
    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

  # Validate reason is one of expected values
  run jq -r '.reason' "$SESSION_END_FIXTURE"
  assert_success
  assert_output --regexp "^(clear|logout|prompt_input_exit|other)$"

  # Validate field types
  assert_json_field_type "$SESSION_END_FIXTURE" ".session_id" "string"
  assert_json_field_type "$SESSION_END_FIXTURE" ".transcript_path" "string"
  assert_json_field_type "$SESSION_END_FIXTURE" ".cwd" "string"
  assert_json_field_type "$SESSION_END_FIXTURE" ".reason" "string"
}

# Test 4: Validate PostToolUse hook JSON schema
# bats test_tags=post-tool-use,schema
@test "should validate PostToolUse hook JSON schema" {
  # Verify fixture exists
  assert_file_exist "$POST_TOOL_USE_FIXTURE"

  # Validate JSON is well-formed
  run jq empty "$POST_TOOL_USE_FIXTURE"
  assert_success

  # Check required fields exist
  assert_json_fields_exist "$POST_TOOL_USE_FIXTURE" \
    ".session_id" \
    ".transcript_path" \
    ".cwd" \
    ".hook_event_name" \
    ".tool_name" \
    ".tool_input" \
    ".tool_response"

  # Validate hook_event_name value
  assert_json_field_equals "$POST_TOOL_USE_FIXTURE" ".hook_event_name" "PostToolUse"

  # Validate tool_name value
  assert_json_field_equals "$POST_TOOL_USE_FIXTURE" ".tool_name" "Bash"

  # Validate session_id is UUID format
  assert_json_field_matches "$POST_TOOL_USE_FIXTURE" ".session_id" \
    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

  # Validate tool_input.command exists and contains git commit
  assert_json_field_exists "$POST_TOOL_USE_FIXTURE" ".tool_input.command"
  assert_json_field_matches "$POST_TOOL_USE_FIXTURE" ".tool_input.command" "git commit"

  # Validate tool_response structure
  assert_json_field_exists "$POST_TOOL_USE_FIXTURE" ".tool_response.exit_code"
  assert_json_field_type "$POST_TOOL_USE_FIXTURE" ".tool_response.exit_code" "number"

  # Validate field types
  assert_json_field_type "$POST_TOOL_USE_FIXTURE" ".session_id" "string"
  assert_json_field_type "$POST_TOOL_USE_FIXTURE" ".transcript_path" "string"
  assert_json_field_type "$POST_TOOL_USE_FIXTURE" ".cwd" "string"
  assert_json_field_type "$POST_TOOL_USE_FIXTURE" ".tool_input" "object"
  assert_json_field_type "$POST_TOOL_USE_FIXTURE" ".tool_response" "object"
}

# Test 5: Validate captured environment variables format
# bats test_tags=environment,capture
@test "should validate captured environment variables format" {
  # Verify fixture exists
  assert_file_exist "$ENV_FIXTURE"

  # Check for expected environment variables
  run grep -q "^CLAUDECODE=" "$ENV_FIXTURE"
  assert_success

  run grep -q "^CLAUDE_PROJECT_DIR=" "$ENV_FIXTURE"
  assert_success

  run grep -q "^CLAUDE_ENV_FILE=" "$ENV_FIXTURE"
  assert_success

  # Verify format (KEY=VALUE pattern)
  run grep -E "^[A-Z_]+=.*$" "$ENV_FIXTURE"
  assert_success
}

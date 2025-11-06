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
STOP_FIXTURE="$FIXTURES_DIR/stop.json"
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

# Test 3: Validate captured environment variables format
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

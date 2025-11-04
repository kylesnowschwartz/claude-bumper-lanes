#!/usr/bin/env bash
set -euo pipefail

# Hook Contract Validation Test Suite
# Purpose: Empirically verify Claude Code hook input schemas match our implementation
# Safety: Uses /tmp directory, can be run repeatedly

# Source test utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/test-output.sh"
source "$SCRIPT_DIR/../lib/test-assertions.sh"
source "$SCRIPT_DIR/../lib/test-env.sh"
source "$SCRIPT_DIR/../lib/hook-test-utils.sh"

# Global test directory
TEST_DIR=""
RESULTS_FILE=""

# Setup test environment
setup_test_env() {
  test_section "Claude Code Hook Contract Validation Test"

  # Create temp test directory
  TEST_DIR=$(create_temp_test_dir "claude-hook-contract-test")
  RESULTS_FILE="$TEST_DIR/validation-results.json"

  # Create capture hooks with environment tracking
  local session_start_hook
  session_start_hook=$(create_capture_hook_with_env "$TEST_DIR" "SessionStart" "session-start")

  local stop_hook
  stop_hook=$(create_stop_capture_hook "$TEST_DIR" "stop")

  # Create hook settings
  local hook_config
  hook_config=$(cat <<EOF
{
  "SessionStart": [
    {
      "hooks": [
        {
          "type": "command",
          "command": "$session_start_hook"
        }
      ]
    }
  ],
  "Stop": [
    {
      "hooks": [
        {
          "type": "command",
          "command": "$stop_hook"
        }
      ]
    }
  ]
}
EOF
)

  create_hook_settings "$TEST_DIR" "$hook_config" >/dev/null
}

# Run test session
run_test_session() {
  test_section "Running test session"

  cd "$TEST_DIR"

  claude -p "Test hook schemas" \
    --model haiku \
    --settings "$TEST_DIR/test-settings.json" \
    --dangerously-skip-permissions \
    >/dev/null 2>&1 || true

  test_info "Test session complete"
}

# Validate SessionStart hook
validate_session_start() {
  local json_file
  json_file=$(find_latest_file "$TEST_DIR" "session-start-*.json")

  if [[ -z "$json_file" ]]; then
    test_fail "No SessionStart capture found"
    return 1
  fi

  validate_session_start_capture "$json_file"
}

# Validate Stop hook
validate_stop() {
  local json_file
  json_file=$(find_latest_file "$TEST_DIR" "stop-*.json")

  if [[ -z "$json_file" ]]; then
    test_fail "No Stop capture found"
    return 1
  fi

  validate_stop_capture "$json_file"
}

# Validate environment variables
validate_env_vars() {
  local env_file
  env_file=$(find_latest_file "$TEST_DIR" "session-start-*.env")

  if [[ -z "$env_file" ]]; then
    test_fail "No environment capture found"
    return 1
  fi

  check_env_vars "$env_file" "CLAUDECODE CLAUDE_PROJECT_DIR CLAUDE_ENV_FILE"
}

# Generate validation report
generate_report() {
  local session_start_result=$1
  local stop_result=$2
  local env_result=$3

  test_section "Validation Report"

  # Write JSON report
  cat >"$RESULTS_FILE" <<EOF
{
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "test_directory": "$TEST_DIR",
  "results": {
    "session_start_schema": $([ "$session_start_result" -eq 0 ] && echo true || echo false),
    "stop_schema": $([ "$stop_result" -eq 0 ] && echo true || echo false),
    "environment_vars": $([ "$env_result" -eq 0 ] && echo true || echo false)
  },
  "captured_schemas": {
    "session_start": $(cat "$(find_latest_file "$TEST_DIR" "session-start-*.json")" 2>/dev/null || echo "null"),
    "stop": $(cat "$(find_latest_file "$TEST_DIR" "stop-*.json")" 2>/dev/null || echo "null")
  }
}
EOF

  echo ""
  jq '.results' "$RESULTS_FILE"

  local all_passed
  all_passed=$(jq -r '.results | to_entries | map(.value) | all' "$RESULTS_FILE")

  echo ""
  if [[ "$all_passed" == "true" ]]; then
    test_pass "ALL VALIDATIONS PASSED"
    echo ""
    test_info "Full report: $RESULTS_FILE"
    test_info "Cleanup: rm -rf $TEST_DIR"
    return 0
  else
    test_fail "SOME VALIDATIONS FAILED"
    echo ""
    test_info "Full report: $RESULTS_FILE"
    test_info "Captured data: $TEST_DIR/*.json"
    test_info "Cleanup: rm -rf $TEST_DIR"
    return 1
  fi
}

# Main execution
main() {
  setup_test_env
  run_test_session

  local session_start_status=0
  local stop_status=0
  local env_status=0

  validate_session_start || session_start_status=$?
  validate_stop || stop_status=$?
  validate_env_vars || env_status=$?

  generate_report $session_start_status $stop_status $env_status
}

# Run main
main

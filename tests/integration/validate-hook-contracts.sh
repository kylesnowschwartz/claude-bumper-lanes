#!/usr/bin/env bash
set -euo pipefail

# Hook Contract Validation Test Suite
# Purpose: Validate hook input JSON schemas against expected contracts
# Uses fixture files instead of running Claude Code

# Source test utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/test-output.sh"
source "$SCRIPT_DIR/../lib/test-assertions.sh"
source "$SCRIPT_DIR/../lib/hook-test-utils.sh"

# Fixture paths
FIXTURES_DIR="$SCRIPT_DIR/../fixtures"
SESSION_START_FIXTURE="$FIXTURES_DIR/session-start.json"
STOP_FIXTURE="$FIXTURES_DIR/stop.json"
ENV_FIXTURE="$FIXTURES_DIR/test-env.txt"

# Validate SessionStart hook schema
validate_session_start() {
  if [[ ! -f "$SESSION_START_FIXTURE" ]]; then
    test_fail "SessionStart fixture not found: $SESSION_START_FIXTURE"
    return 1
  fi

  validate_session_start_capture "$SESSION_START_FIXTURE"
}

# Validate Stop hook schema
validate_stop() {
  if [[ ! -f "$STOP_FIXTURE" ]]; then
    test_fail "Stop fixture not found: $STOP_FIXTURE"
    return 1
  fi

  validate_stop_capture "$STOP_FIXTURE"
}

# Validate environment variables
validate_env_vars() {
  if [[ ! -f "$ENV_FIXTURE" ]]; then
    test_fail "Environment fixture not found: $ENV_FIXTURE"
    return 1
  fi

  check_env_vars "$ENV_FIXTURE" "CLAUDECODE CLAUDE_PROJECT_DIR CLAUDE_ENV_FILE"
}

# Main execution
main() {
  test_section "Hook Contract Validation (Fixture-Based)"

  local session_start_status=0
  local stop_status=0
  local env_status=0

  validate_session_start || session_start_status=$?
  validate_stop || stop_status=$?
  validate_env_vars || env_status=$?

  # Summary
  test_section "Test Summary"

  local total=3
  local passed=0
  [[ $session_start_status -eq 0 ]] && ((passed++)) || true
  [[ $stop_status -eq 0 ]] && ((passed++)) || true
  [[ $env_status -eq 0 ]] && ((passed++)) || true
  local failed=$((total - passed))

  test_summary $passed $failed
}

# Run main
main

#!/usr/bin/env bash
set -euo pipefail

# Hook Contract Validation Test Suite
# Purpose: Empirically verify Claude Code hook input schemas match our implementation
# Safety: Uses /tmp directory, can be run repeatedly

TEST_DIR="/tmp/claude-hook-contract-test-$$"
RESULTS_FILE="$TEST_DIR/validation-results.json"

echo "🧪 Claude Code Hook Contract Validation Test"
echo "=============================================="
echo ""

# Setup test environment
setup_test_env() {
  echo "📁 Creating test environment..."
  mkdir -p "$TEST_DIR"

  # Create capture hooks
  cat >"$TEST_DIR/capture-session-start.sh" <<'HOOK_EOF'
#!/usr/bin/env bash
input=$(cat)
timestamp=$(date +"%Y%m%d_%H%M%S")
echo "$input" > "$TEST_DIR/session-start-${timestamp}.json"
env > "$TEST_DIR/session-start-${timestamp}.env"
exit 0
HOOK_EOF

  cat >"$TEST_DIR/capture-stop.sh" <<'HOOK_EOF'
#!/usr/bin/env bash
input=$(cat)
timestamp=$(date +"%Y%m%d_%H%M%S")
echo "$input" > "$TEST_DIR/stop-${timestamp}.json"
env > "$TEST_DIR/stop-${timestamp}.env"
echo "null"
exit 0
HOOK_EOF

  # Replace $TEST_DIR placeholders with actual value
  sed -i '' "s|\$TEST_DIR|$TEST_DIR|g" "$TEST_DIR/capture-session-start.sh"
  sed -i '' "s|\$TEST_DIR|$TEST_DIR|g" "$TEST_DIR/capture-stop.sh"

  chmod +x "$TEST_DIR"/*.sh

  # Create test settings
  cat >"$TEST_DIR/test-settings.json" <<EOF
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$TEST_DIR/capture-session-start.sh"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$TEST_DIR/capture-stop.sh"
          }
        ]
      }
    ]
  }
}
EOF

  echo "✅ Test environment created at: $TEST_DIR"
}

# Run test session
run_test_session() {
  echo ""
  echo "🚀 Running test session..."
  cd "$TEST_DIR"

  claude -p "Test hook schemas" \
    --model haiku \
    --settings "$TEST_DIR/test-settings.json" \
    --dangerously-skip-permissions \
    >/dev/null 2>&1 || true

  echo "✅ Test session complete"
}

# Validate SessionStart schema
validate_session_start() {
  echo ""
  echo "🔍 Validating SessionStart hook schema..."

  local json_file=$(ls -t "$TEST_DIR"/session-start-*.json 2>/dev/null | head -1)
  if [[ -z "$json_file" ]]; then
    echo "❌ FAIL: No SessionStart capture found"
    return 1
  fi

  local errors=0

  # Check required fields
  for field in session_id transcript_path cwd hook_event_name source; do
    if ! jq -e "has(\"$field\")" "$json_file" >/dev/null 2>&1; then
      echo "   ❌ Missing required field: .$field"
      ((errors++))
    else
      echo "   ✅ Field present: .$field"
    fi
  done

  # Check field values
  local hook_event=$(jq -r '.hook_event_name' "$json_file")
  if [[ "$hook_event" != "SessionStart" ]]; then
    echo "   ❌ hook_event_name should be 'SessionStart', got: $hook_event"
    ((errors++))
  fi

  # Check session_id is UUID format
  local session_id=$(jq -r '.session_id' "$json_file")
  if ! echo "$session_id" | grep -qE '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'; then
    echo "   ❌ session_id not valid UUID format: $session_id"
    ((errors++))
  fi

  if [[ $errors -eq 0 ]]; then
    echo ""
    echo "✅ SessionStart schema validation PASSED"
    return 0
  else
    echo ""
    echo "❌ SessionStart schema validation FAILED ($errors errors)"
    return 1
  fi
}

# Validate Stop schema
validate_stop() {
  echo ""
  echo "🔍 Validating Stop hook schema..."

  local json_file=$(ls -t "$TEST_DIR"/stop-*.json 2>/dev/null | head -1)
  if [[ -z "$json_file" ]]; then
    echo "❌ FAIL: No Stop capture found"
    return 1
  fi

  local errors=0

  # Check required fields
  for field in session_id transcript_path cwd permission_mode hook_event_name stop_hook_active; do
    if ! jq -e "has(\"$field\")" "$json_file" >/dev/null 2>&1; then
      echo "   ❌ Missing required field: .$field"
      ((errors++))
    else
      echo "   ✅ Field present: .$field"
    fi
  done

  # Check field values
  local hook_event=$(jq -r '.hook_event_name' "$json_file")
  if [[ "$hook_event" != "Stop" ]]; then
    echo "   ❌ hook_event_name should be 'Stop', got: $hook_event"
    ((errors++))
  fi

  # Check stop_hook_active is boolean
  local stop_active=$(jq -r '.stop_hook_active' "$json_file")
  if [[ "$stop_active" != "true" && "$stop_active" != "false" ]]; then
    echo "   ❌ stop_hook_active should be boolean, got: $stop_active"
    ((errors++))
  fi

  if [[ $errors -eq 0 ]]; then
    echo ""
    echo "✅ Stop schema validation PASSED"
    return 0
  else
    echo ""
    echo "❌ Stop schema validation FAILED ($errors errors)"
    return 1
  fi
}

# Check environment variables
validate_env_vars() {
  echo ""
  echo "🔍 Validating environment variables..."

  local env_file=$(ls -t "$TEST_DIR"/session-start-*.env 2>/dev/null | head -1)
  if [[ -z "$env_file" ]]; then
    echo "❌ FAIL: No environment capture found"
    return 1
  fi

  local errors=0

  # Check for expected Claude environment variables
  for var in CLAUDECODE CLAUDE_PROJECT_DIR CLAUDE_ENV_FILE; do
    if ! grep -q "^${var}=" "$env_file"; then
      echo "   ❌ Missing environment variable: $var"
      ((errors++))
    else
      echo "   ✅ Variable present: $var"
    fi
  done

  if [[ $errors -eq 0 ]]; then
    echo ""
    echo "✅ Environment validation PASSED"
    return 0
  else
    echo ""
    echo "❌ Environment validation FAILED ($errors errors)"
    return 1
  fi
}

# Generate validation report
generate_report() {
  local session_start_result=$1
  local stop_result=$2
  local env_result=$3

  echo ""
  echo "📊 Validation Report"
  echo "===================="

  # Write JSON report
  cat >"$RESULTS_FILE" <<EOF
{
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "test_directory": "$TEST_DIR",
  "results": {
    "session_start_schema": $([ $session_start_result -eq 0 ] && echo true || echo false),
    "stop_schema": $([ $stop_result -eq 0 ] && echo true || echo false),
    "environment_vars": $([ $env_result -eq 0 ] && echo true || echo false)
  },
  "captured_schemas": {
    "session_start": $(cat $(ls -t "$TEST_DIR"/session-start-*.json 2>/dev/null | head -1) 2>/dev/null || echo "null"),
    "stop": $(cat $(ls -t "$TEST_DIR"/stop-*.json 2>/dev/null | head -1) 2>/dev/null || echo "null")
  }
}
EOF

  echo ""
  cat "$RESULTS_FILE" | jq '.results'

  local all_passed=$(jq -r '.results | to_entries | map(.value) | all' "$RESULTS_FILE")

  echo ""
  if [[ "$all_passed" == "true" ]]; then
    echo "✅ ALL VALIDATIONS PASSED"
    echo ""
    echo "📄 Full report: $RESULTS_FILE"
    echo "🧹 Cleanup: rm -rf $TEST_DIR"
    return 0
  else
    echo "❌ SOME VALIDATIONS FAILED"
    echo ""
    echo "📄 Full report: $RESULTS_FILE"
    echo "🔍 Captured data: $TEST_DIR/*.json"
    echo "🧹 Cleanup: rm -rf $TEST_DIR"
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

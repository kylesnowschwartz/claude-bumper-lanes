#!/usr/bin/env bash
# test-env.sh - Test environment setup and management utilities
# Purpose: Provide reusable functions for test environment creation and cleanup

# Source test output for reporting
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-output.sh"

# Create a temporary test directory
# Usage: create_temp_test_dir prefix
# Returns: Path to created directory (echoed to stdout)
create_temp_test_dir() {
  local prefix="$1"
  local test_dir

  test_dir=$(mktemp -d "/tmp/${prefix}.XXXXXX")
  if [[ ! -d "$test_dir" ]]; then
    test_fail "Failed to create temporary test directory" >&2
    return 1
  fi

  test_info "Created test directory: $test_dir" >&2
  echo "$test_dir"
  return 0
}

# Find the most recent file matching a pattern
# Usage: find_latest_file directory pattern
# Returns: Path to most recent file (echoed to stdout), or empty if not found
find_latest_file() {
  local directory="$1"
  local pattern="$2"

  local latest_file
  latest_file=$(ls -t "$directory"/$pattern 2>/dev/null | head -1)

  if [[ -z "$latest_file" ]]; then
    return 1
  fi

  echo "$latest_file"
  return 0
}

# Create a capture hook script that logs stdin to a file
# Usage: create_capture_hook test_dir hook_name output_prefix [extra_commands]
# Example: create_capture_hook "$TEST_DIR" "SessionStart" "session-start" "env > \$TEST_DIR/session-start-\${timestamp}.env"
create_capture_hook() {
  local test_dir="$1"
  local hook_name="$2"
  local output_prefix="$3"
  local extra_commands="${4:-}"

  local hook_file="$test_dir/capture-${hook_name}.sh"

  cat >"$hook_file" <<HOOK_EOF
#!/usr/bin/env bash
input=\$(cat)
timestamp=\$(date +"%Y%m%d_%H%M%S")
echo "\$input" > "$test_dir/${output_prefix}-\${timestamp}.json"
${extra_commands}
exit 0
HOOK_EOF

  chmod +x "$hook_file"
  echo "$hook_file"
}

# Create a capture hook that also captures environment variables
# Usage: create_capture_hook_with_env test_dir hook_name output_prefix
create_capture_hook_with_env() {
  local test_dir="$1"
  local hook_name="$2"
  local output_prefix="$3"

  create_capture_hook "$test_dir" "$hook_name" "$output_prefix" \
    "env > \"$test_dir/${output_prefix}-\${timestamp}.env\""
}

# Create a Stop capture hook (needs to output "null" to avoid blocking)
# Usage: create_stop_capture_hook test_dir output_prefix
create_stop_capture_hook() {
  local test_dir="$1"
  local output_prefix="$2"

  local hook_file="$test_dir/capture-Stop.sh"

  cat >"$hook_file" <<HOOK_EOF
#!/usr/bin/env bash
input=\$(cat)
timestamp=\$(date +"%Y%m%d_%H%M%S")
echo "\$input" > "$test_dir/${output_prefix}-\${timestamp}.json"
env > "$test_dir/${output_prefix}-\${timestamp}.env"
echo "null"
exit 0
HOOK_EOF

  chmod +x "$hook_file"
  echo "$hook_file"
}

# Create a hooks.json settings file
# Usage: create_hook_settings test_dir hook_configs
# hook_configs: JSON string with hook configurations
create_hook_settings() {
  local test_dir="$1"
  local hook_configs="$2"

  local settings_file="$test_dir/test-settings.json"

  cat >"$settings_file" <<EOF
{
  "hooks": $hook_configs
}
EOF

  echo "$settings_file"
}

# Clean up test directory
# Usage: cleanup_test_dir test_dir
cleanup_test_dir() {
  local test_dir="$1"

  if [[ -d "$test_dir" ]]; then
    rm -rf "$test_dir"
    test_info "Cleaned up test directory: $test_dir" >&2
  fi
}

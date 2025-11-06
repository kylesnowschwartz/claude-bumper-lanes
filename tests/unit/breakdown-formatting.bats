# breakdown-formatting.bats - Tests for threshold breakdown display formatting
#
# Purpose: Validate that format_threshold_breakdown() produces clean, unambiguous output
# Migrated from: test-breakdown-display.sh (legacy custom framework with emoji violations)

# bats file_tags=unit,display,formatting

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'

# Load custom helpers
load '../test_helper/threshold-helpers'
load '../test_helper/json-assertions'

# Test 1: Basic score display (new files only)
# bats test_tags=basic,new-files
@test "should display score and percentage for new files" {
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 100,
  "new_file_additions": 100,
  "edited_file_additions": 0,
  "files_touched": 1,
  "scatter_penalty": 0
}
EOF
  )

  run format_threshold_breakdown "$threshold_data" 200

  assert_success
  assert_output "Threshold: 100/200 points (50%)"
}

# Test 2: Score display with edited files
# bats test_tags=edited-files,percentage
@test "should display correct percentage for edited files" {
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 130,
  "new_file_additions": 0,
  "edited_file_additions": 100,
  "files_touched": 1,
  "scatter_penalty": 0
}
EOF
  )

  run format_threshold_breakdown "$threshold_data" 200

  assert_success
  assert_output "Threshold: 130/200 points (65%)"
}

# Test 3: Different threshold limit
# bats test_tags=percentage-calculation
@test "should calculate percentage with different threshold limits" {
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 230,
  "new_file_additions": 100,
  "edited_file_additions": 100,
  "files_touched": 2,
  "scatter_penalty": 0
}
EOF
  )

  run format_threshold_breakdown "$threshold_data" 400

  assert_success
  assert_output "Threshold: 230/400 points (57%)"
}

# Test 4: Score with scatter penalty
# bats test_tags=scatter,penalty
@test "should show score including scatter penalties" {
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 160,
  "new_file_additions": 100,
  "edited_file_additions": 0,
  "files_touched": 6,
  "scatter_penalty": 60
}
EOF
  )

  run format_threshold_breakdown "$threshold_data" 400

  assert_success
  assert_output "Threshold: 160/400 points (40%)"
}

# Test 5: Lower score scenario (no scatter)
# bats test_tags=basic,percentage
@test "should format score below 100 percent correctly" {
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 100,
  "new_file_additions": 100,
  "edited_file_additions": 0,
  "files_touched": 3,
  "scatter_penalty": 0
}
EOF
  )

  run format_threshold_breakdown "$threshold_data" 400

  assert_success
  assert_output "Threshold: 100/400 points (25%)"
}

# Test 6: Threshold exceeded (>100%)
# bats test_tags=exceeded,over-threshold
@test "should show percentages above 100 percent when threshold exceeded" {
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 501,
  "new_file_additions": 500,
  "edited_file_additions": 0,
  "files_touched": 1,
  "scatter_penalty": 0
}
EOF
  )

  run format_threshold_breakdown "$threshold_data" 400

  assert_success
  assert_output "Threshold: 501/400 points (125%)"
}

# Test 7: Incremental tracking with accumulated score
# This test documents the fix for misleading delta display
# bats test_tags=incremental,accumulation,bug-fix
@test "should display accumulated score not delta in incremental mode" {
  # Simulate what stop.sh sees after incremental tracking
  # Delta this turn: 50 points
  # Accumulated total: 586 points
  local threshold_data
  threshold_data=$(cat <<'EOF'
{
  "weighted_score": 50,
  "new_file_additions": 50,
  "edited_file_additions": 0,
  "files_touched": 1,
  "scatter_penalty": 0,
  "accumulated_score": 586,
  "delta_score": 50
}
EOF
  )

  # Transform like stop.sh does (replace weighted_score with accumulated_score)
  local threshold_data_for_display
  threshold_data_for_display=$(echo "$threshold_data" | jq '.weighted_score = .accumulated_score')

  run format_threshold_breakdown "$threshold_data_for_display" 400

  # Should show correct accumulated total - clean and unambiguous
  assert_success
  assert_output "Threshold: 586/400 points (146%)"
}

# threshold-helpers.bash - Threshold calculation testing utilities for Bats
#
# Purpose: Eliminate duplication of threshold calculation patterns
# Requires: threshold-calculator.sh from bumper-lanes-plugin

# Source the threshold calculator from main plugin
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/../.."
source "$PROJECT_ROOT/bumper-lanes-plugin/hooks/lib/threshold-calculator.sh"

# calculate_and_extract() - Calculate threshold and extract specific field
#
# Args:
#   $1 - baseline tree SHA
#   $2 - current tree SHA
#   $3 - jq field selector (e.g., ".weighted_score", ".new_file_additions")
#
# Returns: Extracted field value on stdout
#
# Usage:
#   score=$(calculate_and_extract "$BASELINE_TREE" "$CURRENT_TREE" ".weighted_score")
#   new_lines=$(calculate_and_extract "$BASELINE_TREE" "$CURRENT_TREE" ".new_file_additions")
calculate_and_extract() {
  local baseline_tree="$1"
  local current_tree="$2"
  local field_selector="$3"

  local result
  result=$(calculate_weighted_threshold "$baseline_tree" "$current_tree")

  echo "$result" | jq -r "$field_selector"
}

# calculate_full_threshold() - Calculate threshold and return full JSON
#
# Args:
#   $1 - baseline tree SHA
#   $2 - current tree SHA
#
# Returns: Full threshold JSON on stdout
#
# Usage:
#   threshold_data=$(calculate_full_threshold "$BASELINE_TREE" "$CURRENT_TREE")
#   score=$(echo "$threshold_data" | jq -r '.weighted_score')
calculate_full_threshold() {
  local baseline_tree="$1"
  local current_tree="$2"

  calculate_weighted_threshold "$baseline_tree" "$current_tree"
}

# calculate_incremental() - Calculate incremental threshold delta
#
# Args:
#   $1 - previous tree SHA
#   $2 - current tree SHA
#   $3 - accumulated score from previous turn
#
# Returns: Full threshold JSON with delta and accumulated scores
#
# Usage:
#   delta=$(calculate_incremental "$prev_tree" "$current_tree" "$accumulated")
#   accumulated=$(echo "$delta" | jq -r '.accumulated_score')
calculate_incremental() {
  local previous_tree="$1"
  local current_tree="$2"
  local accumulated_score="$3"

  calculate_incremental_threshold "$previous_tree" "$current_tree" "$accumulated_score"
}

# assert_threshold_score() - Assert weighted score matches expected value
#
# Args:
#   $1 - baseline tree SHA
#   $2 - current tree SHA
#   $3 - expected score
#
# Usage with bats-assert:
#   assert_threshold_score "$BASELINE_TREE" "$CURRENT_TREE" 200
assert_threshold_score() {
  local baseline_tree="$1"
  local current_tree="$2"
  local expected="$3"

  local actual
  actual=$(calculate_and_extract "$baseline_tree" "$current_tree" ".weighted_score")

  if [[ "$actual" != "$expected" ]]; then
    echo "Expected weighted_score: $expected, got: $actual" >&2
    return 1
  fi
}

# assert_threshold_field() - Assert any threshold field matches expected value
#
# Args:
#   $1 - baseline tree SHA
#   $2 - current tree SHA
#   $3 - field name (e.g., "new_file_additions", "scatter_penalty")
#   $4 - expected value
#
# Usage:
#   assert_threshold_field "$BASELINE_TREE" "$CURRENT_TREE" "new_file_additions" 200
#   assert_threshold_field "$BASELINE_TREE" "$CURRENT_TREE" "scatter_penalty" 70
assert_threshold_field() {
  local baseline_tree="$1"
  local current_tree="$2"
  local field_name="$3"
  local expected="$4"

  local actual
  actual=$(calculate_and_extract "$baseline_tree" "$current_tree" ".$field_name")

  if [[ "$actual" != "$expected" ]]; then
    echo "Expected $field_name: $expected, got: $actual" >&2
    return 1
  fi
}

# format_breakdown_for_display() - Transform delta threshold data for display
#
# Args:
#   $1 - threshold JSON (with weighted_score as delta)
#   $2 - accumulated_score value to display
#   $3 - threshold limit
#
# Returns: Formatted breakdown string
#
# Usage:
#   breakdown=$(format_breakdown_for_display "$threshold_data" "$accumulated" 400)
format_breakdown_for_display() {
  local threshold_data="$1"
  local accumulated_score="$2"
  local threshold_limit="$3"

  # Transform: replace weighted_score with accumulated_score
  local threshold_for_display
  threshold_for_display=$(echo "$threshold_data" | jq ".weighted_score = $accumulated_score")

  # Format using calculator's function
  format_threshold_breakdown "$threshold_for_display" "$threshold_limit"
}

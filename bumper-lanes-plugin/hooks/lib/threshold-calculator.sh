#!/usr/bin/env bash
set -euo pipefail

# threshold-calculator.sh - Weighted threshold calculation for code review (v2)
#
# WHY weighted scoring instead of simple line count:
# - Review burden ≠ lines changed (editing existing code is harder than writing new)
# - Context-switching across files exhausts cognitive capacity non-linearly
# - Deletions reduce complexity, not add it
#
# Research basis:
# - Cisco (2006): 200-400 LOC optimal for 70-90% defect detection, drops dramatically >200
# - Google (2018): 90% of changes touch <10 files (locality is the norm)
# - GitLab: Official guideline ~200 lines per MR for effective review
#
# Formula: weighted_score = (new_additions × 1.0) + (edit_additions × 1.3) + scatter_penalty
#
# WHY 1.3× multiplier for edits:
# - Editing requires understanding existing behavior + integration points
# - More failure modes than greenfield code
# - Conservative middle ground (research suggests 1.2-1.5× range)
#
# WHY scatter penalties:
# - Google data: 90% of changes are localized (<10 files)
# - Cross-file changes signal architectural coupling
# - Context-switching cost grows non-linearly with file count
#
# WHY count additions only:
# - Deletions reduce complexity (code removed = less to maintain)
# - Cisco/Google studies focus on additions as primary review metric

# Scoring configuration
readonly NEW_FILE_WEIGHT=10        # 1.0× baseline (scaled ×10 for integer arithmetic)
readonly EDIT_FILE_WEIGHT=13       # 1.3× penalty (edits harder to review than new code)
readonly SCATTER_LOW_THRESHOLD=6   # Medium penalty starts (typical module = 5 files)
readonly SCATTER_HIGH_THRESHOLD=11 # High penalty starts (Google 90th percentile = 10 files)
readonly SCATTER_PENALTY_LOW=10    # Points/file for 6-10 files
readonly SCATTER_PENALTY_HIGH=30   # Points/file for 11+ files (exponential discouragement)

# calculate_weighted_threshold() - Computes weighted score from git diff data
# Args:
#   $1 - baseline_tree (40-char git tree SHA)
#   $2 - current_tree (40-char git tree SHA)
# Returns: JSON object with weighted score breakdown on stdout
#   {
#     "weighted_score": 165,
#     "new_file_additions": 80,
#     "edited_file_additions": 50,
#     "files_touched": 3,
#     "scatter_penalty": 0
#   }
calculate_weighted_threshold() {
  local baseline_tree=$1
  local current_tree=$2

  if [[ -z "$baseline_tree" ]] || [[ -z "$current_tree" ]]; then
    echo "ERROR: calculate_weighted_threshold requires two tree SHAs" >&2
    return 1
  fi

  # Get per-file diff with file status (Added, Modified, Deleted)
  # Format: <status>\t<additions>\t<deletions>\t<filename>
  local diff_data
  diff_data=$(git diff-tree --numstat --diff-filter=AM "$baseline_tree" "$current_tree" 2>/dev/null | grep -v '^-' || true)

  # Get file status to distinguish new vs edited files
  local file_status
  file_status=$(git diff-tree --name-status --diff-filter=AM "$baseline_tree" "$current_tree" 2>/dev/null || true)

  local new_file_additions=0
  local edited_file_additions=0
  local files_touched=0

  # Process each changed file
  while IFS=$'\t' read -r additions deletions filename; do
    # Skip empty lines and binary files (marked with '-')
    [[ -z "$additions" ]] && continue
    [[ "$additions" == "-" ]] && continue

    # Count additions only (ignore deletions per spec)
    local add_count=$additions

    # Determine file status (A=Added/new, M=Modified/edited)
    local file_status_line
    file_status_line=$(echo "$file_status" | grep -F "$filename" | head -1 || echo "M")
    local status="${file_status_line:0:1}"

    files_touched=$((files_touched + 1))

    if [[ "$status" == "A" ]]; then
      # New file
      new_file_additions=$((new_file_additions + add_count))
    else
      # Modified/edited file
      edited_file_additions=$((edited_file_additions + add_count))
    fi
  done <<<"$diff_data"

  # Calculate scatter penalty based on file count
  # WHY these thresholds:
  # - 1-5 files: Typical single module (code+test+types+docs+config) = no penalty
  # - 6-10 files: Approaching Google's 90th percentile = moderate penalty
  # - 11+ files: Beyond normal locality = exponential discouragement
  local scatter_penalty=0
  if [[ $files_touched -ge $SCATTER_HIGH_THRESHOLD ]]; then
    scatter_penalty=$((files_touched * SCATTER_PENALTY_HIGH))
  elif [[ $files_touched -ge $SCATTER_LOW_THRESHOLD ]]; then
    scatter_penalty=$((files_touched * SCATTER_PENALTY_LOW))
  fi

  # Calculate weighted score
  # WHY integer math: Bash doesn't support floating point, so scale by 10:
  #   1.3× becomes (× 13 ÷ 10), preserving decimal precision
  local new_contribution=$((new_file_additions * NEW_FILE_WEIGHT))
  local edit_contribution=$((edited_file_additions * EDIT_FILE_WEIGHT))
  local total_points=$((new_contribution + edit_contribution))
  local weighted_score=$(((total_points / 10) + scatter_penalty))

  # Return structured JSON
  jq -n \
    --argjson weighted_score "$weighted_score" \
    --argjson new_file_additions "$new_file_additions" \
    --argjson edited_file_additions "$edited_file_additions" \
    --argjson files_touched "$files_touched" \
    --argjson scatter_penalty "$scatter_penalty" \
    '{
      weighted_score: $weighted_score,
      new_file_additions: $new_file_additions,
      edited_file_additions: $edited_file_additions,
      files_touched: $files_touched,
      scatter_penalty: $scatter_penalty
    }'

  return 0
}

# calculate_incremental_threshold() - Calculate delta between consecutive tree states
# Args:
#   $1 - previous_tree (40-char SHA from last tool execution)
#   $2 - current_tree (40-char SHA of current working tree)
#   $3 - accumulated_score (running total from previous operations)
# Returns: JSON with new delta + updated accumulated score
# WHY: Fixes delete+recreate bypass bug by tracking each transition incrementally
calculate_incremental_threshold() {
  local previous_tree=$1
  local current_tree=$2
  local accumulated_score=$3

  # Calculate delta between previous and current (not baseline and current)
  local delta_data
  delta_data=$(calculate_weighted_threshold "$previous_tree" "$current_tree")

  local delta_score
  delta_score=$(echo "$delta_data" | jq -r '.weighted_score')

  # Add delta to accumulated score
  local new_accumulated_score=$((accumulated_score + delta_score))

  # Return combined data with new accumulated total
  echo "$delta_data" | jq \
    --argjson accumulated_score "$new_accumulated_score" \
    --argjson delta_score "$delta_score" \
    '. + {
      accumulated_score: $accumulated_score,
      delta_score: $delta_score
    }'

  return 0
}

# format_threshold_breakdown() - Pretty-print threshold score for user messages
# Args:
#   $1 - threshold_data (JSON output from calculate_weighted_threshold)
#   $2 - threshold_limit (max allowed score)
# Returns: Human-readable score report string on stdout
#
# Note: With incremental tracking, threshold_data contains per-turn delta values
# (new_file_additions, edited_file_additions) that are meaningless when displaying
# accumulated totals. Only the weighted_score and percentage matter for user feedback.
format_threshold_breakdown() {
  local threshold_data=$1
  local threshold_limit=$2

  local weighted_score
  weighted_score=$(echo "$threshold_data" | jq -r '.weighted_score')

  # Calculate percentage
  local threshold_pct
  threshold_pct=$(awk "BEGIN {printf \"%.0f\", ($weighted_score / $threshold_limit) * 100}")

  # Simple score report - percentage tells users what they need to know
  echo "Threshold: $weighted_score/$threshold_limit points ($threshold_pct%)"

  return 0
}

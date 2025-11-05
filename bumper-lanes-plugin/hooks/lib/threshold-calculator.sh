#!/usr/bin/env bash
set -euo pipefail

# threshold-calculator.sh - Weighted threshold calculation for code review (v2)
# Purpose: Calculate review burden using additions-only, edit weighting, and file scatter penalties
#
# This is the PRIMARY threshold system. For simple line-count parsing, see threshold.sh (legacy).
#
# Research foundation:
# - Cisco (2006): 200-400 LOC optimal for 70-90% defect detection
# - Google (2018): 90% of changes touch <10 files, median 24 lines
# - GitLab: Official recommendation ~200 lines per merge request

# Scoring configuration (based on Cisco/Google/GitLab research)
readonly NEW_FILE_WEIGHT=10        # 1.0× (baseline, scaled by 10 for integer math)
readonly EDIT_FILE_WEIGHT=13       # 1.3× (30% penalty for integration complexity)
readonly SCATTER_LOW_THRESHOLD=6   # Files where medium penalty starts
readonly SCATTER_HIGH_THRESHOLD=11 # Files where high penalty starts
readonly SCATTER_PENALTY_LOW=10    # Points per file (6-10 files)
readonly SCATTER_PENALTY_HIGH=30   # Points per file (11+ files)

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
  local scatter_penalty=0
  if [[ $files_touched -ge $SCATTER_HIGH_THRESHOLD ]]; then
    scatter_penalty=$((files_touched * SCATTER_PENALTY_HIGH))
  elif [[ $files_touched -ge $SCATTER_LOW_THRESHOLD ]]; then
    scatter_penalty=$((files_touched * SCATTER_PENALTY_LOW))
  fi
  # 1-5 files: no penalty (0 points)

  # Calculate weighted score
  # new_file_additions × 1.0 + edited_file_additions × 1.3 + scatter_penalty
  # Use integer arithmetic: multiply by 10, apply weights, divide by 10
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

# format_threshold_breakdown() - Pretty-print threshold breakdown for user messages
# Args:
#   $1 - threshold_data (JSON output from calculate_weighted_threshold)
#   $2 - threshold_limit (max allowed score)
# Returns: Human-readable breakdown string on stdout
format_threshold_breakdown() {
  local threshold_data=$1
  local threshold_limit=$2

  local weighted_score
  weighted_score=$(echo "$threshold_data" | jq -r '.weighted_score')
  local new_file_additions
  new_file_additions=$(echo "$threshold_data" | jq -r '.new_file_additions')
  local edited_file_additions
  edited_file_additions=$(echo "$threshold_data" | jq -r '.edited_file_additions')
  local files_touched
  files_touched=$(echo "$threshold_data" | jq -r '.files_touched')
  local scatter_penalty
  scatter_penalty=$(echo "$threshold_data" | jq -r '.scatter_penalty')

  # Calculate percentage
  local threshold_pct
  threshold_pct=$(awk "BEGIN {printf \"%.0f\", ($weighted_score / $threshold_limit) * 100}")

  # Build breakdown message
  local new_pts=$new_file_additions
  local edit_pts=$((edited_file_additions * EDIT_FILE_WEIGHT / 10))

  cat <<EOF
Threshold: $weighted_score/$threshold_limit points ($threshold_pct%)
  • New code: $new_file_additions lines (×1.0 = $new_pts pts)
  • Edited code: $edited_file_additions lines (×1.3 = $edit_pts pts)
  • File scatter: $files_touched files (+$scatter_penalty pts penalty)
EOF

  if [[ $files_touched -ge $SCATTER_LOW_THRESHOLD ]]; then
    echo "  → Changes span multiple modules—consider splitting into focused PRs"
  fi

  return 0
}

#!/usr/bin/env bash
set -euo pipefail

# trip-threshold.sh - Generate enough code to trip the bumper lanes threshold
# Purpose: Manual testing script to verify threshold enforcement behavior

echo "Generating code to trip the 400-point threshold..."
echo ""

# Create a new file with 450 lines (exceeds 400-point limit)
OUTPUT_FILE="generated-code.txt"

echo "Creating $OUTPUT_FILE with 450 lines..."
for i in {1..450}; do
  echo "// Generated line $i - This is test code to trip the bumper lanes threshold" >>"$OUTPUT_FILE"
done

echo ""
echo "✓ Created $OUTPUT_FILE with 450 lines"
echo ""
echo "Expected threshold calculation:"
echo "  - New file: 450 lines × 1.0 = 450 points"
echo "  - Files touched: 1 (no scatter penalty)"
echo "  - Total: 450 points (exceeds 400-point threshold)"
echo ""
echo "Status line should now show: bumper-lanes tripped (red)"
echo "Next Write/Edit tool use should be blocked by PreToolUse hook"

#!/usr/bin/env bash
# Simple smoke test - just verify libraries load without error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Testing library loading..."

if source "$SCRIPT_DIR/../lib/test-output.sh"; then
  echo "[PASS] test-output.sh loads"
else
  echo "[FAIL] test-output.sh fails to load"
  exit 1
fi

if source "$SCRIPT_DIR/../lib/test-assertions.sh"; then
  echo "[PASS] test-assertions.sh loads"
else
  echo "[FAIL] test-assertions.sh fails to load"
  exit 1
fi

if source "$SCRIPT_DIR/../lib/test-env.sh"; then
  echo "[PASS] test-env.sh loads"
else
  echo "[FAIL] test-env.sh fails to load"
  exit 1
fi

if source "$SCRIPT_DIR/../lib/hook-test-utils.sh"; then
  echo "[PASS] hook-test-utils.sh loads"
else
  echo "[FAIL] hook-test-utils.sh fails to load"
  exit 1
fi

echo ""
echo "All libraries loaded successfully"
exit 0

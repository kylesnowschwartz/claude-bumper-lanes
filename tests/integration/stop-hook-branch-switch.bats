#!/usr/bin/env bats
# Test: Stop hook behavior during branch switches
# These tests will FAIL until the fix is implemented

# bats file_tags=integration,stop-hook,branch-switch,failing

load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'
load '../test_helper/bats-file/load'

setup() {
    # Create isolated test environment
    TEST_REPO="$(mktemp -d)/bumper-stop-test-$$"
    mkdir -p "$TEST_REPO"
    cd "$TEST_REPO"

    git init --quiet --initial-branch=main
    git config user.email "test@example.com"
    git config user.name "Test User"

    # Create main branch with initial commit
    echo "main content" > main.txt
    git add main.txt
    git commit --quiet -m "Initial commit on main"

    # Create feature branch with substantial changes (would trip threshold)
    git checkout --quiet -b feature-branch
    for i in {1..15}; do
        printf "%s\n" {1..30} > "feature-file-$i.txt"
    done
    git add .
    git commit --quiet -m "Add feature files"

    # Prepare hook execution environment
    HOOK_DIR="$(cd "$(dirname "$BATS_TEST_DIRNAME")/.." && pwd)/bumper-lanes-plugin/hooks"
    export HOOK_DIR
    export TEST_REPO
}

teardown() {
    if [[ -n "${TEST_REPO:-}" ]] && [[ -d "$TEST_REPO" ]]; then
        cd /
        rm -rf "$TEST_REPO"
    fi
}

# Test: Exact bug reproduction from real scenario
@test "BUGREPRO: Session starts on feature, switches to main, Stop calculates huge diff" {
    cd "$TEST_REPO"

    # Real scenario: Session started on feature-branch
    git checkout --quiet feature-branch
    SESSION_ID="test-session-$$"
    FEATURE_TREE=$(git write-tree)

    mkdir -p .git/bumper-checkpoints

    # But somehow baseline captured main tree (this is the bug!)
    git checkout --quiet main
    MAIN_TREE=$(git write-tree)

    # Session state has baseline from MAIN (simulating session start on main)
    cat > ".git/bumper-checkpoints/session-$SESSION_ID" <<EOF
{
  "session_id": "$SESSION_ID",
  "baseline_tree": "$MAIN_TREE",
  "baseline_branch": "main",
  "previous_tree": "$MAIN_TREE",
  "accumulated_score": 0,
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "threshold_limit": 400,
  "repo_path": "$TEST_REPO",
  "stop_triggered": false
}
EOF

    # Now back on feature branch
    git checkout --quiet feature-branch

    # Run Stop hook
    STOP_INPUT=$(cat <<JSON
{
  "session_id": "$SESSION_ID",
  "cwd": "$TEST_REPO",
  "stop_hook_active": false
}
JSON
)

    run bash "$HOOK_DIR/entrypoints/stop.sh" <<< "$STOP_INPUT"

    # Check accumulated_score
    ACCUMULATED_SCORE=$(jq -r '.accumulated_score' < ".git/bumper-checkpoints/session-$SESSION_ID")

    # BUG CONFIRMED: This will show huge score (all feature branch changes)
    # Expected: 0 (or auto-reset and warn)
    # Actual: 450+ (all feature files counted)

    echo "# DEBUG: accumulated_score = $ACCUMULATED_SCORE" >&3

    # This test DOCUMENTS the bug - it will fail when bug is present
    # FIX: Stop hook should detect baseline is not reachable from HEAD
    #      and either auto-reset or exit with warning
    assert [ "$ACCUMULATED_SCORE" -lt 100 ]
}

# Test: Stop hook auto-resets baseline when unreachable from HEAD
@test "Stop hook: auto-resets baseline when not reachable from current HEAD" {
    cd "$TEST_REPO"

    # Session starts on feature-branch
    git checkout --quiet feature-branch
    SESSION_ID="test-session-$$"
    BASELINE_TREE=$(git write-tree)

    mkdir -p .git/bumper-checkpoints
    cat > ".git/bumper-checkpoints/session-$SESSION_ID" <<EOF
{
  "session_id": "$SESSION_ID",
  "baseline_tree": "$BASELINE_TREE",
  "baseline_branch": "feature-branch",
  "previous_tree": "$BASELINE_TREE",
  "accumulated_score": 0,
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "threshold_limit": 400,
  "repo_path": "$TEST_REPO",
  "stop_triggered": false
}
EOF

    # Switch to main (baseline tree not in main's history)
    git checkout --quiet main

    # Run Stop hook
    STOP_INPUT=$(cat <<JSON
{
  "session_id": "$SESSION_ID",
  "cwd": "$TEST_REPO",
  "stop_hook_active": false
}
JSON
)

    run bash "$HOOK_DIR/entrypoints/stop.sh" <<< "$STOP_INPUT"

    # Should succeed and auto-reset baseline
    assert_success

    # Verify baseline was reset to current tree
    NEW_BASELINE=$(jq -r '.baseline_tree' < ".git/bumper-checkpoints/session-$SESSION_ID")
    CURRENT_TREE=$(git write-tree)

    assert_equal "$NEW_BASELINE" "$CURRENT_TREE"

    # Verify accumulated_score was reset
    ACCUMULATED_SCORE=$(jq -r '.accumulated_score' < ".git/bumper-checkpoints/session-$SESSION_ID")
    assert_equal "$ACCUMULATED_SCORE" "0"
}

# Test: Session state tracks branch name for staleness detection
@test "SessionStart: captures baseline_branch in session state" {
    cd "$TEST_REPO"
    git checkout --quiet feature-branch

    SESSION_ID="test-session-$$"

    # Simulate SessionStart hook
    SESSION_START_INPUT=$(cat <<JSON
{
  "session_id": "$SESSION_ID",
  "cwd": "$TEST_REPO"
}
JSON
)

    run bash "$HOOK_DIR/entrypoints/session-start.sh" <<< "$SESSION_START_INPUT"
    assert_success

    # Verify session state includes baseline_branch
    BASELINE_BRANCH=$(jq -r '.baseline_branch' < ".git/bumper-checkpoints/session-$SESSION_ID")
    assert_equal "$BASELINE_BRANCH" "feature-branch"
}

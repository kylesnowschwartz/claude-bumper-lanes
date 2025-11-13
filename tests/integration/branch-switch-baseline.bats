#!/usr/bin/env bats
# Test: Branch switch baseline staleness detection
# Bug: Session captures baseline on feature branch, switches to main, switches back,
#      and Stop hook calculates diff including all pre-existing feature branch commits.
#
# Scenario that exposes bug:
# 1. Start session on feature-branch (baseline_tree = feature work)
# 2. Switch to main
# 3. Switch back to feature-branch
# 4. Stop hook compares baseline_tree to current_tree
# 5. Diff includes ALL feature branch work (not just session changes)

# bats file_tags=integration,baseline,branch-switch

load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'
load '../test_helper/bats-file/load'

setup() {
    # Create isolated test git repo
    TEST_REPO="$(mktemp -d)/bumper-test-$$"
    mkdir -p "$TEST_REPO"
    cd "$TEST_REPO"

    git init --quiet --initial-branch=main
    git config user.email "test@example.com"
    git config user.name "Test User"

    # Create initial commit on main
    echo "initial" > main-file.txt
    git add main-file.txt
    git commit --quiet -m "Initial commit"
    MAIN_TREE=$(git write-tree)

    # Create feature branch with 10 files (enough to trip threshold if counted)
    git checkout --quiet -b feature-branch
    for i in {1..10}; do
        printf "line 1\nline 2\nline 3\nline 4\nline 5\n" > "feature-$i.txt"
    done
    git add .
    git commit --quiet -m "Add feature files"
    FEATURE_TREE=$(git write-tree)

    # Setup bumper lanes infrastructure
    mkdir -p .git/bumper-checkpoints
    export BUMPER_REPO="$TEST_REPO"
    export MAIN_TREE
    export FEATURE_TREE
}

teardown() {
    if [[ -n "${TEST_REPO:-}" ]] && [[ -d "$TEST_REPO" ]]; then
        cd /
        rm -rf "$TEST_REPO"
    fi
}

# Core bug reproduction test
@test "Bug reproduction: branch switch causes phantom diff accumulation" {
    cd "$TEST_REPO"

    # Step 1: Session starts on feature-branch (simulating SessionStart hook)
    SESSION_ID="test-$(date +%s)"
    BASELINE_TREE="$FEATURE_TREE"

    cat > ".git/bumper-checkpoints/session-$SESSION_ID" <<EOF
{
  "session_id": "$SESSION_ID",
  "baseline_tree": "$BASELINE_TREE",
  "previous_tree": "$BASELINE_TREE",
  "accumulated_score": 0,
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "threshold_limit": 400,
  "repo_path": "$TEST_REPO",
  "stop_triggered": false
}
EOF

    # Step 2: User asks Claude to switch to main
    git checkout --quiet main

    # Step 3: User asks Claude to switch back to feature
    git checkout --quiet feature-branch

    # Step 4: Calculate what Stop hook would see
    CURRENT_TREE=$(git write-tree)

    # Trees should match (no uncommitted changes)
    assert_equal "$BASELINE_TREE" "$CURRENT_TREE"

    # But if PreToolUse ran during branch switches and accumulated diffs...
    # Calculate diff between main and feature (what might get counted)
    PHANTOM_DIFF=$(git diff-tree --numstat "$MAIN_TREE" "$FEATURE_TREE" | awk '{sum += $1 + $2} END {print sum+0}')

    # This shows the phantom diff exists (50 lines across 10 files)
    assert [ "$PHANTOM_DIFF" -gt 40 ]

    # EXPECTED: accumulated_score should be 0 (no changes in session)
    # ACTUAL: If hooks ran on branch switch, accumulated_score would be non-zero
}

# Test detection: baseline tree is not in current branch history
@test "Detect when baseline tree is not reachable from current HEAD" {
    cd "$TEST_REPO"

    # Baseline captured on feature-branch
    BASELINE_TREE="$FEATURE_TREE"

    # Now on main branch (feature commits not in history)
    git checkout --quiet main

    # Find commit with baseline tree
    BASELINE_COMMIT=$(git log --all --format="%H %T" | grep "$BASELINE_TREE" | awk '{print $1}' | head -1)

    # Check if baseline commit is ancestor of current HEAD
    CURRENT_HEAD=$(git rev-parse HEAD)

    if git merge-base --is-ancestor "$BASELINE_COMMIT" "$CURRENT_HEAD" 2>/dev/null; then
        IS_ANCESTOR=true
    else
        IS_ANCESTOR=false
    fi

    # Baseline is NOT an ancestor when we switch from feature to main
    assert_equal "$IS_ANCESTOR" "false"
}

# Test proposed fix: track branch name to detect switches
@test "Branch name tracking would detect switch scenario" {
    cd "$TEST_REPO"

    # Simulated session state with branch tracking
    BASELINE_BRANCH="feature-branch"
    git checkout --quiet "$BASELINE_BRANCH"

    # Switch to main
    git checkout --quiet main
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

    # Branch changed - this should trigger staleness warning
    assert_not_equal "$BASELINE_BRANCH" "$CURRENT_BRANCH"

    # Switch back
    git checkout --quiet feature-branch
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

    # Branch matches again, but we need to track that switch happened
    assert_equal "$BASELINE_BRANCH" "$CURRENT_BRANCH"
}

# Test proposed fix: reset baseline after branch switch
@test "Baseline should reset when returning to different tree" {
    skip "Proposed fix not yet implemented"

    # Proposed behavior:
    # 1. SessionStart captures: baseline_tree + baseline_branch
    # 2. PreToolUse before git checkout: save current_branch
    # 3. Stop hook: check if current_branch != baseline_branch
    # 4. If branch changed:
    #    a. Check if baseline_tree reachable from HEAD
    #    b. If not: auto-reset baseline to current tree
    #    c. If yes: warn user baseline may be stale
}

# Test: Verify tree comparison logic (baseline check)
@test "Tree comparison correctly identifies identical trees" {
    cd "$TEST_REPO"
    git checkout --quiet feature-branch

    TREE1=$(git write-tree)
    TREE2=$(git write-tree)

    # Same tree should have no diff
    DIFF_STAT=$(git diff-tree --numstat "$TREE1" "$TREE2" | wc -l | tr -d ' ')
    assert_equal "$DIFF_STAT" "0"
}

# Test: Verify tree comparison finds real changes
@test "Tree comparison detects actual file changes" {
    cd "$TEST_REPO"
    git checkout --quiet feature-branch

    TREE_BEFORE=$(git write-tree)

    # Make a change
    echo "new line" >> feature-1.txt
    git add feature-1.txt

    TREE_AFTER=$(git write-tree)

    # Should detect change
    assert_not_equal "$TREE_BEFORE" "$TREE_AFTER"

    DIFF_LINES=$(git diff-tree --numstat "$TREE_BEFORE" "$TREE_AFTER" | awk '{print $1}')
    assert_equal "$DIFF_LINES" "1"
}

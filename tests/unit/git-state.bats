# git-state.bats - Tests for git-state.sh library functions
#
# Purpose: Verify capture_tree() and compute_diff() work correctly
#          in edge cases including empty repositories
# Related to: Bug fix for "Failed to capture tree" in repos without commits

# bats file_tags=unit,git-state,capture-tree

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'

# Source the library under test
LIB_DIR="$BATS_TEST_DIRNAME/../../bumper-lanes-plugin/hooks/lib"

setup() {
  TEST_REPO="$BATS_TEST_TMPDIR/test-repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
}

teardown() {
  if [[ -n "$TEST_REPO" ]] && [[ -d "$TEST_REPO" ]]; then
    cd /
    rm -rf "$TEST_REPO"
  fi
}

# Test: capture_tree works in empty repo (no commits)
# This was a bug - git read-tree HEAD corrupted the temp index
# bats test_tags=bug-fix,empty-repo
@test "capture_tree should work in repo with no commits" {
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Source library
  source "$LIB_DIR/git-state.sh"

  # Should succeed and return empty tree SHA
  run capture_tree
  assert_success

  # Empty tree has well-known SHA
  assert_output "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
}

# Test: capture_tree includes untracked files in empty repo
# bats test_tags=empty-repo,untracked
@test "capture_tree should include untracked files in empty repo" {
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Create untracked file
  echo "hello world" > test.txt

  source "$LIB_DIR/git-state.sh"

  run capture_tree
  assert_success

  # Should NOT be empty tree since we have untracked file
  refute_output "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

  # Should be a valid 40-char SHA
  assert_output --regexp '^[0-9a-f]{40}$'
}

# Test: capture_tree works in repo with commits
# bats test_tags=normal-repo
@test "capture_tree should work in repo with commits" {
  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"
  git commit --allow-empty -m "initial" -q

  echo "content" > file.txt

  source "$LIB_DIR/git-state.sh"

  run capture_tree
  assert_success
  assert_output --regexp '^[0-9a-f]{40}$'
}

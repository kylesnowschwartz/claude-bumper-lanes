# git-test-helpers.bash - Git repository testing utilities for Bats
#
# Purpose: Eliminate duplication of git repo setup/teardown patterns
# Used by: All unit tests requiring git repositories

# Global variables (set by helpers, used by tests)
TEST_REPO=""
BASELINE_TREE=""
CURRENT_TREE=""

# setup_git_test_repo() - Create temporary git repository with baseline commit
#
# Creates a fresh git repo in $BATS_TEST_TMPDIR with:
# - Configured user.name and user.email
# - Initial empty commit
# - Baseline tree SHA captured in $BASELINE_TREE
#
# Sets global variables:
#   TEST_REPO - Path to created repository
#   BASELINE_TREE - Git tree SHA of baseline commit
#
# Usage:
#   @test "example" {
#     setup_git_test_repo
#     # TEST_REPO and BASELINE_TREE are now available
#   }
setup_git_test_repo() {
  TEST_REPO="$BATS_TEST_TMPDIR/test-repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"

  git init -q
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Create baseline with empty commit
  git commit --allow-empty -m "initial" -q
  BASELINE_TREE=$(git write-tree)
}

# cleanup_git_test_repo() - Remove test repository
#
# Cleans up test repository created by setup_git_test_repo().
# Safe to call multiple times or if repo doesn't exist.
#
# Usage:
#   teardown() {
#     cleanup_git_test_repo
#   }
cleanup_git_test_repo() {
  if [[ -n "$TEST_REPO" ]] && [[ -d "$TEST_REPO" ]]; then
    cd /
    rm -rf "$TEST_REPO"
  fi
  TEST_REPO=""
  BASELINE_TREE=""
  CURRENT_TREE=""
}

# add_file_to_repo() - Create file with N lines in test repository
#
# Args:
#   $1 - filename (relative to repo root)
#   $2 - number of lines to generate
#   $3 - line prefix (optional, default: "line")
#
# Usage:
#   add_file_to_repo "test.txt" 50
#   add_file_to_repo "data.txt" 100 "data"
add_file_to_repo() {
  local filename="$1"
  local line_count="$2"
  local prefix="${3:-line}"

  for i in $(seq 1 "$line_count"); do
    echo "$prefix $i"
  done >"$filename"
}

# add_files_to_repo() - Create multiple files with specified line counts
#
# Args: Pairs of filename and line count
#   $1 - first filename
#   $2 - first file line count
#   $3 - second filename (optional)
#   $4 - second file line count (optional)
#   ... additional pairs
#
# Stages all created files with git add.
# Captures resulting tree in $CURRENT_TREE.
#
# Usage:
#   add_files_to_repo "file1.txt" 80 "file2.txt" 50 "file3.txt" 70
add_files_to_repo() {
  while [[ $# -gt 0 ]]; do
    local filename="$1"
    local line_count="$2"

    if [[ -z "$filename" ]] || [[ -z "$line_count" ]]; then
      break
    fi

    add_file_to_repo "$filename" "$line_count"
    shift 2
  done

  git add .
  CURRENT_TREE=$(git write-tree)
}

# append_to_file() - Append N lines to existing file
#
# Args:
#   $1 - filename (relative to repo root)
#   $2 - number of lines to append
#   $3 - starting line number (optional, default: 1)
#   $4 - line prefix (optional, default: "new line")
#
# Usage:
#   append_to_file "existing.txt" 50 11 "added"
append_to_file() {
  local filename="$1"
  local line_count="$2"
  local start_num="${3:-1}"
  local prefix="${4:-new line}"

  local end_num=$((start_num + line_count - 1))

  for i in $(seq "$start_num" "$end_num"); do
    echo "$prefix $i"
  done >>"$filename"
}

# commit_staged_changes() - Commit staged changes and update CURRENT_TREE
#
# Args:
#   $1 - commit message (optional, default: "test commit")
#
# Note: Stages all changes automatically before committing
#
# Usage:
#   commit_staged_changes "Update file"
commit_staged_changes() {
  local message="${1:-test commit}"

  git add .
  git commit -m "$message" -q
  CURRENT_TREE=$(git write-tree)
}

# stage_and_capture_tree() - Stage all changes and capture tree SHA
#
# Convenience function that runs git add . and updates CURRENT_TREE.
# Does NOT commit.
#
# Usage:
#   add_file_to_repo "new.txt" 50
#   stage_and_capture_tree
stage_and_capture_tree() {
  git add .
  CURRENT_TREE=$(git write-tree)
}

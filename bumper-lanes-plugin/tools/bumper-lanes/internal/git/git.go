// Package git wraps the git plumbing bumper-lanes relies on: working-tree
// snapshots, branch and HEAD queries, and tree-level merges. All functions
// operate on the repository containing the process working directory.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsRepo checks if current directory is in a git repository.
func IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// CaptureTree captures the current working tree as a git tree SHA.
// Uses a temporary index to avoid modifying the real staging area.
//
// diff-viz's diff.CaptureCurrentTree implements the same temp-index
// snapshot independently, in its own repository.
//
// TODO: consolidate the two into one shared implementation; that needs
// diff-viz to depend on this module (or vice versa) across the repo
// boundary.
func CaptureTree() (string, error) {
	// Create temp index file
	tmpIndex, err := os.CreateTemp("", "git-index-*")
	if err != nil {
		return "", err
	}
	tmpIndexPath := tmpIndex.Name()
	tmpIndex.Close()
	defer os.Remove(tmpIndexPath)

	// Helper to run git commands with GIT_INDEX_FILE set
	gitWithTempIndex := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndexPath)
		return cmd
	}

	// Initialize temp index with HEAD tree (or empty if no commits)
	headRef, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil && len(headRef) > 0 {
		if err := gitWithTempIndex("read-tree", strings.TrimSpace(string(headRef))).Run(); err != nil {
			return "", fmt.Errorf("read-tree HEAD: %w", err)
		}
	} else {
		if err := gitWithTempIndex("read-tree", "--empty").Run(); err != nil {
			return "", fmt.Errorf("read-tree --empty: %w", err)
		}
	}

	// Add tracked file changes (staged and unstaged). A repo with no
	// tracked files yet (fresh init, or HEAD^{tree} empty) reports a
	// pathspec error here - benign, since there is nothing to update.
	if out, err := gitWithTempIndex("add", "-u", ".").CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "did not match any file") {
			return "", fmt.Errorf("add -u: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}

	// Add untracked files (respecting .gitignore). -z / NUL-splitting keeps
	// paths raw: without it, git C-quotes non-ASCII, quote, and backslash
	// characters (default core.quotePath) and the quoted string is not a
	// path `git add` can find, silently failing the whole capture.
	lsCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z")
	untrackedOutput, err := lsCmd.Output()
	if err != nil {
		return "", fmt.Errorf("ls-files --others: %w", err)
	}
	for _, path := range strings.Split(strings.TrimRight(string(untrackedOutput), "\x00"), "\x00") {
		if path != "" {
			if err := gitWithTempIndex("add", "--", path).Run(); err != nil {
				return "", fmt.Errorf("add %q: %w", path, err)
			}
		}
	}

	// Write tree from temp index
	writeCmd := gitWithTempIndex("write-tree")
	output, err := writeCmd.Output()
	if err != nil {
		return "", err
	}

	treeSHA := strings.TrimSpace(string(output))
	if treeSHA == "" {
		return "", fmt.Errorf("empty tree SHA")
	}

	return treeSHA, nil
}

// CurrentBranch returns the current branch name, or empty string if detached.
func CurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(output))
	if branch == "HEAD" {
		return "" // Detached HEAD
	}
	return branch
}

// HeadCommit returns the commit SHA of HEAD, or "none" on an unborn
// branch (fresh repo with no commits). The non-empty sentinel lets callers
// distinguish "recorded on an unborn branch" from "never recorded".
func HeadCommit() string {
	output, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "none"
	}
	return strings.TrimSpace(string(output))
}

// HeadTree returns the tree SHA of HEAD.
// Returns empty string if HEAD doesn't exist (empty repo) or on error.
func HeadTree() string {
	return Tree("HEAD")
}

// Tree resolves a commit-ish to its tree SHA, or "" on failure.
func Tree(commitish string) string {
	out, err := exec.Command("git", "rev-parse", commitish+"^{tree}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// MergeTrees 3-way-merges trees in a temp index and returns the resulting
// tree SHA. An unmergeable index (conflicting change to the same path)
// fails; callers keep whatever tree they started with.
func MergeTrees(base, ours, theirs string) (string, error) {
	tmpIndex, err := os.CreateTemp("", "bumper-rebase-index-*")
	if err != nil {
		return "", err
	}
	tmpIndexPath := tmpIndex.Name()
	tmpIndex.Close()
	// read-tree -m refuses a pre-existing zero-byte index file; git must
	// create the index itself at this path.
	os.Remove(tmpIndexPath)
	defer os.Remove(tmpIndexPath)

	gitWithTempIndex := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndexPath)
		return cmd
	}

	if out, err := gitWithTempIndex("read-tree", "-i", "-m", base, ours, theirs).CombinedOutput(); err != nil {
		return "", fmt.Errorf("read-tree: %s", strings.TrimSpace(string(out)))
	}
	out, err := gitWithTempIndex("write-tree").Output()
	if err != nil {
		return "", fmt.Errorf("write-tree: %w", err)
	}
	tree := strings.TrimSpace(string(out))
	if tree == "" {
		return "", fmt.Errorf("empty tree from merge")
	}
	return tree, nil
}

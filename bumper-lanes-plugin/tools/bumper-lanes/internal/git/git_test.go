package git

import (
	"os"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/testutil"
)

func TestIsRepo(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)

	t.Chdir(tmpDir)

	if !IsRepo() {
		t.Error("IsRepo() = false, want true inside a real git repo")
	}

	nonRepoDir := t.TempDir()
	testutil.IsolateGitEnv(t, nonRepoDir)
	t.Chdir(nonRepoDir)
	if IsRepo() {
		t.Error("IsRepo() = true, want false outside any git repo")
	}
}

func TestCurrentBranch(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)

	t.Chdir(tmpDir)

	branch := CurrentBranch()
	if branch != "main" {
		t.Errorf("CurrentBranch() = %q, want %q", branch, "main")
	}
}

// TestCaptureTree covers the tree-capture happy paths and the error path
// for discarded git failures.
func TestCaptureTree(t *testing.T) {
	t.Run("fresh repo with no tracked files still captures a tree", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.IsolateGitEnv(t, tmpDir)
		testutil.SetupTempGitRepo(t, tmpDir)

		t.Chdir(tmpDir)

		tree, err := CaptureTree()
		if err != nil {
			t.Fatalf("CaptureTree() error = %v, want nil", err)
		}
		if tree == "" {
			t.Error("CaptureTree() returned empty tree SHA")
		}
	})

	t.Run("untracked file is captured in the tree", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.IsolateGitEnv(t, tmpDir)
		testutil.SetupTempGitRepo(t, tmpDir)

		t.Chdir(tmpDir)

		if err := os.WriteFile(tmpDir+"/new.txt", []byte("hello"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		before, err := CaptureTree()
		if err != nil {
			t.Fatalf("CaptureTree() error = %v, want nil", err)
		}

		headTree := HeadTree()
		if before == headTree {
			t.Error("CaptureTree() did not include the untracked file")
		}
	})

	t.Run("error surfaces outside a git repository", func(t *testing.T) {
		tmpDir := t.TempDir() // not a git repo
		testutil.IsolateGitEnv(t, tmpDir)

		t.Chdir(tmpDir)

		if _, err := CaptureTree(); err == nil {
			t.Error("CaptureTree() error = nil, want non-nil outside a git repo")
		}
	})

	t.Run("untracked file with non-ASCII name is captured", func(t *testing.T) {
		// Under the default core.quotePath, `git ls-files` C-quotes
		// non-ASCII filenames; without -z/NUL-splitting, `git add` is
		// handed the quoted string instead of the real path and fails.
		tmpDir := t.TempDir()
		testutil.IsolateGitEnv(t, tmpDir)
		testutil.SetupTempGitRepo(t, tmpDir)

		t.Chdir(tmpDir)

		if err := os.WriteFile(tmpDir+"/café.md", []byte("hello"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		tree, err := CaptureTree()
		if err != nil {
			t.Fatalf("CaptureTree() error = %v, want nil", err)
		}

		headTree := HeadTree()
		if tree == headTree {
			t.Error("CaptureTree() did not include the non-ASCII untracked file")
		}
	})
}

package git

import (
	"os"
	"os/exec"
	"testing"
)

// isolateGitEnv clears GIT_DIR/GIT_WORK_TREE and caps directory discovery at
// tmpDir, so an inherited git environment (parent repo, git-hook process)
// can't hijack setupTempGitRepo's init or flip IsRepo's negative case.
func isolateGitEnv(t *testing.T, tmpDir string) {
	t.Helper()
	t.Setenv("GIT_CEILING_DIRECTORIES", tmpDir)
	unsetEnv(t, "GIT_DIR")
	unsetEnv(t, "GIT_WORK_TREE")
}

// unsetEnv removes an env var for the duration of the test, restoring its
// original value (or absence) on cleanup. Setting it to "" instead (via
// t.Setenv) is not equivalent: git treats an empty GIT_DIR as a real,
// invalid value rather than as unset.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, wasSet := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if wasSet {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	})
}

// setupTempGitRepo initializes a git repo in tmpDir with an initial commit.
func setupTempGitRepo(t *testing.T, tmpDir string) {
	t.Helper()

	// Use -b main to ensure consistent branch name across different git configs
	// (CI may default to "master" while tests use "main")
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	configCmds := [][]string{
		{"config", "user.name", "Test"},
		{"config", "user.email", "test@example.com"},
	}
	for _, args := range configCmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
}

func TestIsRepo(t *testing.T) {
	tmpDir := t.TempDir()
	isolateGitEnv(t, tmpDir)
	setupTempGitRepo(t, tmpDir)

	t.Chdir(tmpDir)

	if !IsRepo() {
		t.Error("IsRepo() = false, want true inside a real git repo")
	}

	nonRepoDir := t.TempDir()
	isolateGitEnv(t, nonRepoDir)
	t.Chdir(nonRepoDir)
	if IsRepo() {
		t.Error("IsRepo() = true, want false outside any git repo")
	}
}

func TestCurrentBranch(t *testing.T) {
	tmpDir := t.TempDir()
	isolateGitEnv(t, tmpDir)
	setupTempGitRepo(t, tmpDir)

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
		isolateGitEnv(t, tmpDir)
		setupTempGitRepo(t, tmpDir)

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
		isolateGitEnv(t, tmpDir)
		setupTempGitRepo(t, tmpDir)

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
		isolateGitEnv(t, tmpDir)

		t.Chdir(tmpDir)

		if _, err := CaptureTree(); err == nil {
			t.Error("CaptureTree() error = nil, want non-nil outside a git repo")
		}
	})
}

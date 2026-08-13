package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTempGitRepo initializes a git repo in tmpDir
func setupTempGitRepo(t *testing.T, tmpDir string) {
	t.Helper()

	// Use -b main to ensure consistent branch name across different git configs
	// (CI may default to "master" while tests use "main")
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git identity (required for CI environments)
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

	// Create initial commit so we have HEAD
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create bumper-checkpoints directory
	checkpointDir := filepath.Join(tmpDir, ".git", "bumper-checkpoints")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
}

// isolateGitEnv clears GIT_DIR/GIT_WORK_TREE and caps directory discovery at
// tmpDir, so an inherited git environment (parent repo, git-hook process)
// can't hijack setupTempGitRepo's init or flip git.IsRepo's negative case.
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

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

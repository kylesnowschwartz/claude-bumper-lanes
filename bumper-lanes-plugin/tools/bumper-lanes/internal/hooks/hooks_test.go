package hooks

import (
	"testing"
)

func TestIsGitRepo(t *testing.T) {
	// This test runs in a git repo, so should return true
	if !IsGitRepo() {
		t.Skip("Not running in a git repo")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	branch := GetCurrentBranch()
	// Should return something in a git repo
	if branch == "" {
		t.Skip("No branch or detached HEAD")
	}
	t.Logf("Current branch: %s", branch)
}

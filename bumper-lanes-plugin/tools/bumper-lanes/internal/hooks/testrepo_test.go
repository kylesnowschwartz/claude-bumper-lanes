package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/testutil"
)

// setupTempGitRepo initializes a git repo in tmpDir and creates the
// bumper-checkpoints directory the hooks package's session state lives in.
func setupTempGitRepo(t *testing.T, tmpDir string) {
	t.Helper()
	testutil.SetupTempGitRepo(t, tmpDir)

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
	testutil.IsolateGitEnv(t, tmpDir)
}

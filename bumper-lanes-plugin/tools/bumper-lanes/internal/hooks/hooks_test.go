package hooks

import (
	"os"
	"testing"
)

// isolateGitEnv clears GIT_DIR/GIT_WORK_TREE and caps directory discovery at
// tmpDir, so an inherited git environment (parent repo, git-hook process)
// can't hijack setupTempGitRepo's init or flip IsGitRepo's negative case.
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

func TestIsGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	isolateGitEnv(t, tmpDir)
	setupTempGitRepo(t, tmpDir)

	t.Chdir(tmpDir)

	if !IsGitRepo() {
		t.Error("IsGitRepo() = false, want true inside a real git repo")
	}

	nonRepoDir := t.TempDir()
	isolateGitEnv(t, nonRepoDir)
	t.Chdir(nonRepoDir)
	if IsGitRepo() {
		t.Error("IsGitRepo() = true, want false outside any git repo")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	tmpDir := t.TempDir()
	isolateGitEnv(t, tmpDir)
	setupTempGitRepo(t, tmpDir)

	t.Chdir(tmpDir)

	branch := GetCurrentBranch()
	if branch != "main" {
		t.Errorf("GetCurrentBranch() = %q, want %q", branch, "main")
	}
}

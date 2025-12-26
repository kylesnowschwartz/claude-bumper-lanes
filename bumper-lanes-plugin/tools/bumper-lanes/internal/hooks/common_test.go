package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetGitDiffTreePath(t *testing.T) {
	// Save original env and restore after test
	origPluginRoot := os.Getenv("CLAUDE_PLUGIN_ROOT")
	defer os.Setenv("CLAUDE_PLUGIN_ROOT", origPluginRoot)

	t.Run("CLAUDE_PLUGIN_ROOT takes priority when binary exists", func(t *testing.T) {
		// Create a temp directory with a fake binary
		tmpDir := t.TempDir()
		binDir := filepath.Join(tmpDir, "bin")
		if err := os.MkdirAll(binDir, 0755); err != nil {
			t.Fatal(err)
		}
		fakeBin := filepath.Join(binDir, "git-diff-tree")
		if err := os.WriteFile(fakeBin, []byte("fake"), 0755); err != nil {
			t.Fatal(err)
		}

		os.Setenv("CLAUDE_PLUGIN_ROOT", tmpDir)
		got := GetGitDiffTreePath()
		want := fakeBin

		if got != want {
			t.Errorf("GetGitDiffTreePath() = %q, want %q", got, want)
		}
	})

	t.Run("CLAUDE_PLUGIN_ROOT ignored if binary doesn't exist", func(t *testing.T) {
		// Set to a directory without the binary
		tmpDir := t.TempDir()
		os.Setenv("CLAUDE_PLUGIN_ROOT", tmpDir)

		got := GetGitDiffTreePath()
		// Should NOT return the tmpDir path since binary doesn't exist
		if got == filepath.Join(tmpDir, "bin", "git-diff-tree") {
			t.Errorf("GetGitDiffTreePath() returned non-existent path: %q", got)
		}
	})

	t.Run("empty CLAUDE_PLUGIN_ROOT uses fallback", func(t *testing.T) {
		os.Setenv("CLAUDE_PLUGIN_ROOT", "")

		got := GetGitDiffTreePath()
		// Should return something (either PATH lookup or relative path)
		if got == "" {
			t.Error("GetGitDiffTreePath() returned empty string")
		}
	})

	t.Run("finds binary via relative path in development", func(t *testing.T) {
		os.Setenv("CLAUDE_PLUGIN_ROOT", "")

		// Save current dir and change to project root
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)

		// Navigate up to find project root (where bumper-lanes-plugin exists)
		// From internal/hooks/ need to go up 5 levels to reach project root
		// This is fragile but tests real development scenario
		for i := 0; i < 6; i++ {
			if _, err := os.Stat("bumper-lanes-plugin/bin/git-diff-tree"); err == nil {
				got := GetGitDiffTreePath()
				if got != "bumper-lanes-plugin/bin/git-diff-tree" {
					t.Logf("Found via different path: %s", got)
				}
				return
			}
			os.Chdir("..")
		}
		t.Skip("Could not find project root with binary")
	})
}

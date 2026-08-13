package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPluginOptionsPrecedence covers the userConfig env source: legacy
// global file < plugin options (CLAUDE_PLUGIN_OPTION_*) < repo file.
func TestPluginOptionsPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)
	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("plugin options apply when no files set the value", func(t *testing.T) {
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "250")
		t.Setenv("CLAUDE_PLUGIN_OPTION_ON_TRIP", "review")
		t.Setenv("CLAUDE_PLUGIN_OPTION_MAX_AUTO_REVIEWS", "-1")
		t.Setenv("CLAUDE_PLUGIN_OPTION_TRIPWIRES_BLOCK_AUTO_REVIEW", "true")

		if got := LoadThreshold(); got != 250 {
			t.Errorf("LoadThreshold() = %d, want 250 (plugin option)", got)
		}
		if got := LoadOnTrip(); got != OnTripReview {
			t.Errorf("LoadOnTrip() = %q, want review", got)
		}
		if got := LoadMaxAutoReviews(); got != UnlimitedAutoReviews {
			t.Errorf("LoadMaxAutoReviews() = %d, want -1", got)
		}
		if !LoadTripwiresBlockAutoReview() {
			t.Error("LoadTripwiresBlockAutoReview() = false, want true (plugin option)")
		}
	})

	t.Run("plugin options override the legacy global file", func(t *testing.T) {
		globalDir := filepath.Join(xdg, "bumper-lanes")
		os.MkdirAll(globalDir, 0755)
		os.WriteFile(filepath.Join(globalDir, "config.json"), []byte(`{"threshold": 900}`), 0644)
		defer os.RemoveAll(globalDir)
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "250")

		if got := LoadThreshold(); got != 250 {
			t.Errorf("LoadThreshold() = %d, want 250 (plugin option over global file)", got)
		}
	})

	t.Run("repo file overrides plugin options", func(t *testing.T) {
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "250")
		os.WriteFile(repoPath, []byte(`{"threshold": 75}`), 0644)
		defer os.Remove(repoPath)

		if got := LoadThreshold(); got != 75 {
			t.Errorf("LoadThreshold() = %d, want 75 (repo file wins)", got)
		}
	})

	t.Run("unparseable numeric option is ignored", func(t *testing.T) {
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "lots")

		if got := LoadThreshold(); got != DefaultThreshold {
			t.Errorf("LoadThreshold() = %d, want default for junk env", got)
		}
	})
}

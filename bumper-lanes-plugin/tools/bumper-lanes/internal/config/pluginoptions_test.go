package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/testutil"
)

// TestPluginOptionsPrecedence covers the userConfig env source: plugin
// options (CLAUDE_PLUGIN_OPTION_*) < repo file.
func TestPluginOptionsPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	t.Chdir(tmpDir)
	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("plugin options apply when no files set the value", func(t *testing.T) {
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "250")
		t.Setenv("CLAUDE_PLUGIN_OPTION_ON_TRIP", "review")
		t.Setenv("CLAUDE_PLUGIN_OPTION_MAX_AUTO_REVIEWS", "-1")
		t.Setenv("CLAUDE_PLUGIN_OPTION_TRIPWIRES_BLOCK_AUTO_REVIEW", "true")

		if got := loadSettings().Threshold; got != 250 {
			t.Errorf("loadSettings().Threshold = %d, want 250 (plugin option)", got)
		}
		if got := loadSettings().OnTrip; got != OnTripReview {
			t.Errorf("loadSettings().OnTrip = %q, want review", got)
		}
		if got := loadSettings().MaxAutoReviews; got != UnlimitedAutoReviews {
			t.Errorf("loadSettings().MaxAutoReviews = %d, want -1", got)
		}
		if !loadSettings().TripwiresBlockAutoReview {
			t.Error("loadSettings().TripwiresBlockAutoReview = false, want true (plugin option)")
		}
	})

	t.Run("repo file overrides plugin options", func(t *testing.T) {
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "250")
		os.WriteFile(repoPath, []byte(`{"threshold": 75}`), 0644)
		defer os.Remove(repoPath)

		if got := loadSettings().Threshold; got != 75 {
			t.Errorf("loadSettings().Threshold = %d, want 75 (repo file wins)", got)
		}
	})

	t.Run("unparseable numeric option is ignored", func(t *testing.T) {
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "lots")

		if got := loadSettings().Threshold; got != DefaultThreshold {
			t.Errorf("loadSettings().Threshold = %d, want default for junk env", got)
		}
	})
}

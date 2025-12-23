package hooks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

func TestView(t *testing.T) {
	// Skip if not in a git repo
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("updates both session state and config file", func(t *testing.T) {
		// Create a temp git repo for testing
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		// Save and restore current dir
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create session state
		sessionID := "test-view-session"
		sess, err := state.New(sessionID, "abc123", "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Call View
		mode := "collapsed"
		err = View(sessionID, mode)
		if err != nil {
			t.Fatalf("View() error = %v", err)
		}

		// Verify session state updated
		reloaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("Failed to reload session: %v", err)
		}
		if reloaded.ViewMode != mode {
			t.Errorf("Session ViewMode = %q, want %q", reloaded.ViewMode, mode)
		}

		// Verify config file updated
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		configPath := filepath.Join(string(gitDir[:len(gitDir)-1]), "bumper-config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("Failed to parse config: %v", err)
		}

		if cfg["default_view_mode"] != mode {
			t.Errorf("Config default_view_mode = %q, want %q", cfg["default_view_mode"], mode)
		}
	})

	t.Run("rejects invalid mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		sessionID := "test-view-invalid"
		sess, err := state.New(sessionID, "abc123", "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		err = View(sessionID, "invalid_mode_xyz")
		if err == nil {
			t.Error("View() with invalid mode should return error")
		}
	})
}

func TestPersistViewModeToConfig(t *testing.T) {
	t.Run("creates config file if not exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		err := persistViewModeToConfig("tree")
		if err != nil {
			t.Fatalf("persistViewModeToConfig() error = %v", err)
		}

		// Verify file was created
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		configPath := filepath.Join(string(gitDir[:len(gitDir)-1]), "bumper-config.json")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("Config file was not created")
		}
	})

	t.Run("preserves existing config values", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create existing config with other values
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		configPath := filepath.Join(string(gitDir[:len(gitDir)-1]), "bumper-config.json")
		existingCfg := map[string]interface{}{
			"threshold":         300,
			"default_view_mode": "tree",
		}
		data, _ := json.Marshal(existingCfg)
		os.WriteFile(configPath, data, 0644)

		// Update view mode
		err := persistViewModeToConfig("icicle")
		if err != nil {
			t.Fatalf("persistViewModeToConfig() error = %v", err)
		}

		// Verify threshold preserved, view mode updated
		newData, _ := os.ReadFile(configPath)
		var cfg map[string]interface{}
		json.Unmarshal(newData, &cfg)

		if cfg["threshold"].(float64) != 300 {
			t.Errorf("threshold = %v, want 300", cfg["threshold"])
		}
		if cfg["default_view_mode"] != "icicle" {
			t.Errorf("default_view_mode = %v, want icicle", cfg["default_view_mode"])
		}
	})
}

// setupTempGitRepo initializes a git repo in tmpDir
func setupTempGitRepo(t *testing.T, tmpDir string) {
	t.Helper()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Create initial commit so we have HEAD
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create bumper-checkpoints directory
	checkpointDir := filepath.Join(tmpDir, ".git", "bumper-checkpoints")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
}

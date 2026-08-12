package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigLoading verifies config loading from .bumper-lanes.json.
func TestConfigLoading(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real global config

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("default when no config file exists", func(t *testing.T) {
		os.Remove(repoPath)

		got := LoadThreshold()
		if got != DefaultThreshold {
			t.Errorf("LoadThreshold() = %d, want %d (default)", got, DefaultThreshold)
		}
	})

	t.Run("config overrides default threshold", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"threshold": 200}`), 0644)
		defer os.Remove(repoPath)

		got := LoadThreshold()
		if got != 200 {
			t.Errorf("LoadThreshold() = %d, want 200 (config)", got)
		}
	})

	t.Run("unknown keys are ignored", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"threshold": 200, "default_view_mode": "sparkline-tree"}`), 0644)
		defer os.Remove(repoPath)

		got := LoadThreshold()
		if got != 200 {
			t.Errorf("LoadThreshold() = %d, want 200 (unknown keys ignored)", got)
		}
	})
}

// TestLoadStatuslineAutoSetup verifies status line setup is opt-in.
func TestLoadStatuslineAutoSetup(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real global config

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("defaults to false", func(t *testing.T) {
		os.Remove(repoPath)
		if LoadStatuslineAutoSetup() {
			t.Error("LoadStatuslineAutoSetup() = true, want false (opt-in default)")
		}
	})

	t.Run("repo config opts in", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"statusline_auto_setup": true}`), 0644)
		defer os.Remove(repoPath)
		if !LoadStatuslineAutoSetup() {
			t.Error("LoadStatuslineAutoSetup() = false, want true (config)")
		}
	})

	t.Run("explicit false stays false", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"statusline_auto_setup": false}`), 0644)
		defer os.Remove(repoPath)
		if LoadStatuslineAutoSetup() {
			t.Error("LoadStatuslineAutoSetup() = true, want false")
		}
	})
}

// TestLoadResetOn verifies the commit auto-reset policy loading.
func TestLoadResetOn(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real global config

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("defaults to commit", func(t *testing.T) {
		os.Remove(repoPath)
		if got := LoadResetOn(); got != ResetOnCommit {
			t.Errorf("LoadResetOn() = %q, want %q (default)", got, ResetOnCommit)
		}
	})

	t.Run("loads configured policies", func(t *testing.T) {
		for _, policy := range []string{ResetOnCommit, ResetOnVerifiedCommit, ResetOnHuman} {
			os.WriteFile(repoPath, []byte(`{"reset_on": "`+policy+`"}`), 0644)
			if got := LoadResetOn(); got != policy {
				t.Errorf("LoadResetOn() = %q, want %q", got, policy)
			}
		}
		os.Remove(repoPath)
	})

	t.Run("unknown value falls back to default", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"reset_on": "never-heard-of-it"}`), 0644)
		defer os.Remove(repoPath)
		if got := LoadResetOn(); got != DefaultResetOn {
			t.Errorf("LoadResetOn() = %q, want %q (fallback)", got, DefaultResetOn)
		}
	})
}

// TestGitWorktreeDetection verifies GetGitDir works in worktrees.
// Worktrees have .git as a file pointing to the real git dir.
func TestGitWorktreeDetection(t *testing.T) {
	// Create main repo
	mainRepo := t.TempDir()
	setupGitRepo(t, mainRepo)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(mainRepo)

	// Create a worktree
	worktreeDir := t.TempDir()
	cmd := exec.Command("git", "worktree", "add", worktreeDir, "-b", "test-branch")
	if err := cmd.Run(); err != nil {
		t.Skipf("git worktree not supported: %v", err)
	}
	defer exec.Command("git", "worktree", "remove", worktreeDir).Run()

	os.Chdir(worktreeDir)

	t.Run("GetGitDir returns worktree-specific git dir", func(t *testing.T) {
		gitDir, err := GetGitDir()
		if err != nil {
			t.Fatalf("GetGitDir() error = %v", err)
		}

		// Should be .git/worktrees/<name>, not .git
		if !strings.Contains(gitDir, "worktrees") {
			t.Errorf("GetGitDir() = %q, want path containing 'worktrees'", gitDir)
		}
	})
}

// TestEmptyRepoNoHEAD verifies we don't crash on repos without commits.
func TestEmptyRepoNoHEAD(t *testing.T) {
	tmpDir := t.TempDir()

	// git init but NO commit
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real global config

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	t.Run("LoadThreshold succeeds without HEAD", func(t *testing.T) {
		// Should not panic, should return default
		got := LoadThreshold()
		if got != DefaultThreshold {
			t.Errorf("LoadThreshold() = %d, want %d", got, DefaultThreshold)
		}
	})

	t.Run("LoadResetOn succeeds without HEAD", func(t *testing.T) {
		got := LoadResetOn()
		if got != DefaultResetOn {
			t.Errorf("LoadResetOn() = %q, want %q", got, DefaultResetOn)
		}
	})

	t.Run("GetGitDir succeeds without HEAD", func(t *testing.T) {
		gitDir, err := GetGitDir()
		if err != nil {
			t.Fatalf("GetGitDir() error = %v", err)
		}
		if gitDir == "" {
			t.Error("GetGitDir() returned empty string")
		}
	})
}

func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "init")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	configJSON := `{"threshold": 300, "reset_on": "human"}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := loadConfigFile(configPath)
	if err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}

	if cfg.Threshold == nil || *cfg.Threshold != 300 {
		t.Errorf("Threshold = %v, want 300", cfg.Threshold)
	}
	if cfg.ResetOn != "human" {
		t.Errorf("ResetOn = %q, want %q", cfg.ResetOn, "human")
	}
}

func TestUnknownKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	t.Run("names keys the schema does not understand", func(t *testing.T) {
		os.WriteFile(configPath, []byte(`{"threshold": 300, "default_view_mode": "tree", "show_diff_viz": true}`), 0644)
		got := UnknownKeys(configPath)
		want := []string{"default_view_mode", "show_diff_viz"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("UnknownKeys() = %v, want %v", got, want)
		}
	})

	t.Run("nil for all-known keys", func(t *testing.T) {
		os.WriteFile(configPath, []byte(`{"threshold": 300, "reset_on": "human"}`), 0644)
		if got := UnknownKeys(configPath); got != nil {
			t.Errorf("UnknownKeys() = %v, want nil", got)
		}
	})

	t.Run("nil for missing or invalid file", func(t *testing.T) {
		if got := UnknownKeys(filepath.Join(tmpDir, "nope.json")); got != nil {
			t.Errorf("UnknownKeys(missing) = %v, want nil", got)
		}
		os.WriteFile(configPath, []byte("not json"), 0644)
		if got := UnknownKeys(configPath); got != nil {
			t.Errorf("UnknownKeys(invalid) = %v, want nil", got)
		}
	})
}

func TestLoadConfigFile_Missing(t *testing.T) {
	_, err := loadConfigFile("/nonexistent/path/config.json")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoadConfigFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad-config.json")

	if err := os.WriteFile(configPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := loadConfigFile(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestGlobalConfigLoading verifies global config as fallback for repo config.
func TestGlobalConfigLoading(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create temp XDG config dir
	xdgDir := t.TempDir()
	globalConfigDir := filepath.Join(xdgDir, "bumper-lanes")
	os.MkdirAll(globalConfigDir, 0755)
	globalConfigPath := filepath.Join(globalConfigDir, "config.json")

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		}
	}()
	os.Chdir(tmpDir)
	os.Setenv("XDG_CONFIG_HOME", xdgDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("global config used when no repo config", func(t *testing.T) {
		os.Remove(repoPath)
		os.WriteFile(globalConfigPath, []byte(`{"threshold": 100}`), 0644)
		defer os.Remove(globalConfigPath)

		got := LoadThreshold()
		if got != 100 {
			t.Errorf("LoadThreshold() = %d, want 100 (global)", got)
		}
	})

	t.Run("repo config overrides global config", func(t *testing.T) {
		os.WriteFile(globalConfigPath, []byte(`{"threshold": 100}`), 0644)
		os.WriteFile(repoPath, []byte(`{"threshold": 200}`), 0644)
		defer os.Remove(globalConfigPath)
		defer os.Remove(repoPath)

		got := LoadThreshold()
		if got != 200 {
			t.Errorf("LoadThreshold() = %d, want 200 (repo overrides global)", got)
		}
	})

	t.Run("merge: repo threshold with global reset policy", func(t *testing.T) {
		os.WriteFile(globalConfigPath, []byte(`{"threshold": 100, "reset_on": "human"}`), 0644)
		os.WriteFile(repoPath, []byte(`{"threshold": 200}`), 0644) // only threshold, no reset policy
		defer os.Remove(globalConfigPath)
		defer os.Remove(repoPath)

		gotThreshold := LoadThreshold()
		gotResetOn := LoadResetOn()
		if gotThreshold != 200 {
			t.Errorf("LoadThreshold() = %d, want 200 (repo)", gotThreshold)
		}
		if gotResetOn != ResetOnHuman {
			t.Errorf("LoadResetOn() = %q, want %q (global)", gotResetOn, ResetOnHuman)
		}
	})

	t.Run("global threshold 0 disables enforcement", func(t *testing.T) {
		os.Remove(repoPath)
		os.WriteFile(globalConfigPath, []byte(`{"threshold": 0}`), 0644)
		defer os.Remove(globalConfigPath)

		got := LoadThreshold()
		if got != 0 {
			t.Errorf("LoadThreshold() = %d, want 0 (disabled via global)", got)
		}
		if !IsDisabled(got) {
			t.Error("IsDisabled() = false, want true")
		}
	})

	t.Run("default when neither config exists", func(t *testing.T) {
		os.Remove(repoPath)
		os.Remove(globalConfigPath)

		got := LoadThreshold()
		if got != DefaultThreshold {
			t.Errorf("LoadThreshold() = %d, want %d (default)", got, DefaultThreshold)
		}
	})
}

func TestGetGlobalConfigPath(t *testing.T) {
	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		origXDG := os.Getenv("XDG_CONFIG_HOME")
		defer func() {
			if origXDG == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", origXDG)
			}
		}()

		os.Setenv("XDG_CONFIG_HOME", "/custom/config")
		got := GetGlobalConfigPath()
		want := "/custom/config/bumper-lanes/config.json"
		if got != want {
			t.Errorf("GetGlobalConfigPath() = %q, want %q", got, want)
		}
	})

	t.Run("falls back to ~/.config when XDG not set", func(t *testing.T) {
		origXDG := os.Getenv("XDG_CONFIG_HOME")
		defer func() {
			if origXDG == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", origXDG)
			}
		}()

		os.Unsetenv("XDG_CONFIG_HOME")
		got := GetGlobalConfigPath()
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config", "bumper-lanes", "config.json")
		if got != want {
			t.Errorf("GetGlobalConfigPath() = %q, want %q", got, want)
		}
	})
}

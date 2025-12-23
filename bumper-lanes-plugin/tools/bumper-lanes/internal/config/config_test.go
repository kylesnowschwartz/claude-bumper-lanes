package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigPriorityChain verifies Personal > Repo > Default precedence.
// This is documented behavior that's easy to break during refactoring.
func TestConfigPriorityChain(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	gitDir := filepath.Join(tmpDir, ".git")
	personalPath := filepath.Join(gitDir, "bumper-config.json")
	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("default when no config files exist", func(t *testing.T) {
		// Clean slate
		os.Remove(personalPath)
		os.Remove(repoPath)

		got := LoadThreshold()
		if got != DefaultThreshold {
			t.Errorf("LoadThreshold() = %d, want %d (default)", got, DefaultThreshold)
		}
	})

	t.Run("repo config overrides default", func(t *testing.T) {
		os.Remove(personalPath)
		os.WriteFile(repoPath, []byte(`{"threshold": 200}`), 0644)
		defer os.Remove(repoPath)

		got := LoadThreshold()
		if got != 200 {
			t.Errorf("LoadThreshold() = %d, want 200 (repo config)", got)
		}
	})

	t.Run("personal config overrides repo config", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"threshold": 200}`), 0644)
		os.WriteFile(personalPath, []byte(`{"threshold": 500}`), 0644)
		defer os.Remove(repoPath)
		defer os.Remove(personalPath)

		got := LoadThreshold()
		if got != 500 {
			t.Errorf("LoadThreshold() = %d, want 500 (personal > repo)", got)
		}
	})

	t.Run("view mode priority chain", func(t *testing.T) {
		os.Remove(personalPath)
		os.Remove(repoPath)

		// Default
		if got := LoadViewMode(); got != DefaultViewMode {
			t.Errorf("LoadViewMode() = %q, want %q (default)", got, DefaultViewMode)
		}

		// Repo overrides default
		os.WriteFile(repoPath, []byte(`{"default_view_mode": "collapsed"}`), 0644)
		if got := LoadViewMode(); got != "collapsed" {
			t.Errorf("LoadViewMode() = %q, want %q (repo)", got, "collapsed")
		}

		// Personal overrides repo
		os.WriteFile(personalPath, []byte(`{"default_view_mode": "icicle"}`), 0644)
		if got := LoadViewMode(); got != "icicle" {
			t.Errorf("LoadViewMode() = %q, want %q (personal > repo)", got, "icicle")
		}
	})

	t.Run("invalid view mode falls through to next priority", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"default_view_mode": "collapsed"}`), 0644)
		os.WriteFile(personalPath, []byte(`{"default_view_mode": "INVALID"}`), 0644)
		defer os.Remove(repoPath)
		defer os.Remove(personalPath)

		// Personal is invalid, should fall through to repo
		got := LoadViewMode()
		if got != "collapsed" {
			t.Errorf("LoadViewMode() = %q, want %q (skip invalid personal)", got, "collapsed")
		}
	})
}

// TestGitWorktreeDetection verifies config works in worktrees.
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

	t.Run("personal config isolated per worktree", func(t *testing.T) {
		gitDir, _ := GetGitDir()
		personalPath := filepath.Join(gitDir, "bumper-config.json")
		os.WriteFile(personalPath, []byte(`{"threshold": 999}`), 0644)
		defer os.Remove(personalPath)

		got := LoadThreshold()
		if got != 999 {
			t.Errorf("LoadThreshold() in worktree = %d, want 999", got)
		}

		// Main repo should NOT see this config
		os.Chdir(mainRepo)
		mainThreshold := LoadThreshold()
		if mainThreshold == 999 {
			t.Error("Main repo saw worktree's personal config - isolation broken")
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

	t.Run("LoadViewMode succeeds without HEAD", func(t *testing.T) {
		got := LoadViewMode()
		if got != DefaultViewMode {
			t.Errorf("LoadViewMode() = %q, want %q", got, DefaultViewMode)
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

func TestIsValidMode(t *testing.T) {
	tests := []struct {
		mode  string
		valid bool
	}{
		{"tree", true},
		{"collapsed", true},
		{"smart", true},
		{"topn", true},
		{"icicle", true},
		{"brackets", true},
		{"invalid", false},
		{"", false},
		{"TREE", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			if got := isValidMode(tt.mode); got != tt.valid {
				t.Errorf("isValidMode(%q) = %v, want %v", tt.mode, got, tt.valid)
			}
		})
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	configJSON := `{"threshold": 300, "default_view_mode": "collapsed"}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := loadConfigFile(configPath)
	if err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}

	if cfg.Threshold != 300 {
		t.Errorf("Threshold = %d, want 300", cfg.Threshold)
	}
	if cfg.DefaultViewMode != "collapsed" {
		t.Errorf("DefaultViewMode = %q, want %q", cfg.DefaultViewMode, "collapsed")
	}
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


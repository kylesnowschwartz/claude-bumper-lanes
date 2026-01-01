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

	t.Run("view mode loading", func(t *testing.T) {
		os.Remove(repoPath)

		// Default
		if got := LoadViewMode(); got != DefaultViewMode {
			t.Errorf("LoadViewMode() = %q, want %q (default)", got, DefaultViewMode)
		}

		// Config overrides default
		os.WriteFile(repoPath, []byte(`{"default_view_mode": "sparkline-tree"}`), 0644)
		defer os.Remove(repoPath)
		if got := LoadViewMode(); got != "sparkline-tree" {
			t.Errorf("LoadViewMode() = %q, want %q (config)", got, "sparkline-tree")
		}
	})

	t.Run("invalid view mode falls through to default", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"default_view_mode": "INVALID"}`), 0644)
		defer os.Remove(repoPath)

		got := LoadViewMode()
		if got != DefaultViewMode {
			t.Errorf("LoadViewMode() = %q, want %q (invalid should use default)", got, DefaultViewMode)
		}
	})

	t.Run("view opts loading", func(t *testing.T) {
		os.Remove(repoPath)

		// Default is empty
		if got := LoadViewOpts(); got != "" {
			t.Errorf("LoadViewOpts() = %q, want empty (default)", got)
		}

		// Config provides opts
		os.WriteFile(repoPath, []byte(`{"default_view_opts": "--width 80 --depth 3"}`), 0644)
		defer os.Remove(repoPath)
		if got := LoadViewOpts(); got != "--width 80 --depth 3" {
			t.Errorf("LoadViewOpts() = %q, want '--width 80 --depth 3' (config)", got)
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
		// Valid modes (diff-viz v2.0.0)
		{"tree", true},
		{"smart", true},
		{"sparkline-tree", true},
		{"hotpath", true},
		{"icicle", true},
		{"brackets", true},
		{"gauge", true},
		{"depth", true},
		{"stat", true},
		// Removed modes (no longer valid)
		{"collapsed", false},
		{"topn", false},
		// Other invalid
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

	configJSON := `{"threshold": 300, "default_view_mode": "sparkline-tree"}`
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
	if cfg.DefaultViewMode != "sparkline-tree" {
		t.Errorf("DefaultViewMode = %q, want %q", cfg.DefaultViewMode, "sparkline-tree")
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

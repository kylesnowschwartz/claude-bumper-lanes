package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/testutil"
)

// TestConfigLoading verifies config loading from .bumper-lanes.json.
func TestConfigLoading(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real global config
	t.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("default when no config file exists", func(t *testing.T) {
		os.Remove(repoPath)

		got := loadSettings().Threshold
		if got != DefaultThreshold {
			t.Errorf("loadSettings().Threshold = %d, want %d (default)", got, DefaultThreshold)
		}
	})

	t.Run("config overrides default threshold", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"threshold": 200}`), 0644)
		defer os.Remove(repoPath)

		got := loadSettings().Threshold
		if got != 200 {
			t.Errorf("loadSettings().Threshold = %d, want 200 (config)", got)
		}
	})

	t.Run("unknown keys are ignored", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"threshold": 200, "default_view_mode": "sparkline-tree"}`), 0644)
		defer os.Remove(repoPath)

		got := loadSettings().Threshold
		if got != 200 {
			t.Errorf("loadSettings().Threshold = %d, want 200 (unknown keys ignored)", got)
		}
	})
}

// TestLoadWarnings verifies that a malformed config file and an
// unparseable plugin env var surface a warning instead of being silently
// indistinguishable from an absent config.
func TestLoadWarnings(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("no config file produces no warning", func(t *testing.T) {
		os.Remove(repoPath)
		if _, got := Load(); len(got) != 0 {
			t.Errorf("Load() warnings = %v, want none for absent config", got)
		}
	})

	t.Run("malformed config file produces a warning", func(t *testing.T) {
		os.WriteFile(repoPath, []byte("not json"), 0644)
		defer os.Remove(repoPath)

		_, got := Load()
		if len(got) != 1 {
			t.Fatalf("Load() warnings = %v, want exactly 1 warning", got)
		}
		if !strings.Contains(got[0], repoPath) {
			t.Errorf("warning %q does not name the config path %q", got[0], repoPath)
		}
	})

	t.Run("unparseable plugin env value produces a warning", func(t *testing.T) {
		os.Remove(repoPath)
		t.Setenv("CLAUDE_PLUGIN_OPTION_THRESHOLD", "not-a-number")

		_, got := Load()
		if len(got) != 1 || !strings.Contains(got[0], "CLAUDE_PLUGIN_OPTION_THRESHOLD") {
			t.Errorf("Load() warnings = %v, want a warning naming CLAUDE_PLUGIN_OPTION_THRESHOLD", got)
		}
	})
}

// TestLoadStatuslineAutoSetup verifies status line setup is opt-in.
func TestLoadStatuslineAutoSetup(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real global config
	t.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("defaults to false", func(t *testing.T) {
		os.Remove(repoPath)
		if loadSettings().StatuslineAutoSetup {
			t.Error("loadSettings().StatuslineAutoSetup = true, want false (opt-in default)")
		}
	})

	t.Run("repo config opts in", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"statusline_auto_setup": true}`), 0644)
		defer os.Remove(repoPath)
		if !loadSettings().StatuslineAutoSetup {
			t.Error("loadSettings().StatuslineAutoSetup = false, want true (config)")
		}
	})

	t.Run("explicit false stays false", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"statusline_auto_setup": false}`), 0644)
		defer os.Remove(repoPath)
		if loadSettings().StatuslineAutoSetup {
			t.Error("loadSettings().StatuslineAutoSetup = true, want false")
		}
	})
}

// TestLoadResetOn verifies the commit auto-reset policy loading.
func TestLoadResetOn(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real global config
	t.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("defaults to commit", func(t *testing.T) {
		os.Remove(repoPath)
		if got := loadSettings().ResetOn; got != ResetOnCommit {
			t.Errorf("loadSettings().ResetOn = %q, want %q (default)", got, ResetOnCommit)
		}
	})

	t.Run("loads configured policies", func(t *testing.T) {
		for _, policy := range []string{ResetOnCommit, ResetOnVerifiedCommit, ResetOnHuman} {
			os.WriteFile(repoPath, []byte(`{"reset_on": "`+policy+`"}`), 0644)
			if got := loadSettings().ResetOn; got != policy {
				t.Errorf("loadSettings().ResetOn = %q, want %q", got, policy)
			}
		}
		os.Remove(repoPath)
	})

	t.Run("unknown value falls back to default", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"reset_on": "never-heard-of-it"}`), 0644)
		defer os.Remove(repoPath)
		if got := loadSettings().ResetOn; got != DefaultResetOn {
			t.Errorf("loadSettings().ResetOn = %q, want %q (fallback)", got, DefaultResetOn)
		}
	})
}

// TestGitWorktreeDetection verifies GetGitDir works in worktrees.
// Worktrees have .git as a file pointing to the real git dir.
func TestGitWorktreeDetection(t *testing.T) {
	// Create main repo
	mainRepo := t.TempDir()
	testutil.IsolateGitEnv(t, mainRepo)
	testutil.SetupTempGitRepo(t, mainRepo)

	t.Chdir(mainRepo)

	// Create a worktree
	worktreeDir := t.TempDir()
	cmd := exec.Command("git", "worktree", "add", worktreeDir, "-b", "test-branch")
	if err := cmd.Run(); err != nil {
		t.Skipf("git worktree not supported: %v", err)
	}
	defer exec.Command("git", "worktree", "remove", worktreeDir).Run()

	t.Chdir(worktreeDir)

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
	t.Chdir(tmpDir)

	t.Run("LoadThreshold succeeds without HEAD", func(t *testing.T) {
		// Should not panic, should return default
		got := loadSettings().Threshold
		if got != DefaultThreshold {
			t.Errorf("loadSettings().Threshold = %d, want %d", got, DefaultThreshold)
		}
	})

	t.Run("LoadResetOn succeeds without HEAD", func(t *testing.T) {
		got := loadSettings().ResetOn
		if got != DefaultResetOn {
			t.Errorf("loadSettings().ResetOn = %q, want %q", got, DefaultResetOn)
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

// TestLegacyGlobalConfigIgnored verifies the pre-v5 global file is no longer
// read and its presence surfaces a warning.
func TestLegacyGlobalConfigIgnored(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)

	// Create temp XDG config dir
	xdgDir := t.TempDir()
	globalConfigDir := filepath.Join(xdgDir, "bumper-lanes")
	os.MkdirAll(globalConfigDir, 0755)
	globalConfigPath := filepath.Join(globalConfigDir, "config.json")

	t.Chdir(tmpDir)
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("legacy file values are ignored, warning names the file", func(t *testing.T) {
		os.Remove(repoPath)
		os.WriteFile(globalConfigPath, []byte(`{"threshold": 100}`), 0644)
		defer os.Remove(globalConfigPath)

		s, warnings := Load()
		if s.Threshold != DefaultThreshold {
			t.Errorf("Load().Threshold = %d, want %d (legacy file ignored)", s.Threshold, DefaultThreshold)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], globalConfigPath) {
			t.Errorf("Load() warnings = %v, want one warning naming %s", warnings, globalConfigPath)
		}
	})

	t.Run("no warning when legacy file is absent", func(t *testing.T) {
		os.Remove(repoPath)
		os.Remove(globalConfigPath)

		s, warnings := Load()
		if s.Threshold != DefaultThreshold {
			t.Errorf("Load().Threshold = %d, want %d (default)", s.Threshold, DefaultThreshold)
		}
		if len(warnings) != 0 {
			t.Errorf("Load() warnings = %v, want none", warnings)
		}
	})
}

func TestLegacyGlobalConfigPathHelper(t *testing.T) {
	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/custom/config")
		got := legacyGlobalConfigPath()
		want := "/custom/config/bumper-lanes/config.json"
		if got != want {
			t.Errorf("legacyGlobalConfigPath() = %q, want %q", got, want)
		}
	})

	t.Run("falls back to ~/.config when XDG not set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		got := legacyGlobalConfigPath()
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config", "bumper-lanes", "config.json")
		if got != want {
			t.Errorf("legacyGlobalConfigPath() = %q, want %q", got, want)
		}
	})
}

// TestLoadTripwires verifies tripwires are opt-in: disabled when unset,
// with "defaults" expanding to the recommended lists.
func TestLoadTripwires(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.IsolateGitEnv(t, tmpDir)
	testutil.SetupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(tmpDir)

	repoPath := filepath.Join(tmpDir, ".bumper-lanes.json")

	t.Run("disabled when unset", func(t *testing.T) {
		os.Remove(repoPath)
		s := loadSettings()
		if len(s.TripwirePaths) != 0 || len(s.TripwirePatterns) != 0 {
			t.Errorf("TripwirePaths = %v, TripwirePatterns = %v, want both empty (opt-in)", s.TripwirePaths, s.TripwirePatterns)
		}
	})

	t.Run("defaults entry expands to recommended lists", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"tripwire_paths": ["custom/**", "defaults"], "tripwire_patterns": ["defaults"]}`), 0644)
		defer os.Remove(repoPath)
		s := loadSettings()
		wantPaths := append([]string{"custom/**"}, RecommendedTripwirePaths...)
		if !slices.Equal(s.TripwirePaths, wantPaths) {
			t.Errorf("TripwirePaths = %v, want %v", s.TripwirePaths, wantPaths)
		}
		if !slices.Equal(s.TripwirePatterns, RecommendedTripwirePatterns) {
			t.Errorf("TripwirePatterns = %v, want %v", s.TripwirePatterns, RecommendedTripwirePatterns)
		}
	})

	t.Run("explicit list used verbatim", func(t *testing.T) {
		os.WriteFile(repoPath, []byte(`{"tripwire_paths": ["only/this/**"]}`), 0644)
		defer os.Remove(repoPath)
		s := loadSettings()
		if !slices.Equal(s.TripwirePaths, []string{"only/this/**"}) {
			t.Errorf("TripwirePaths = %v, want [only/this/**]", s.TripwirePaths)
		}
	})
}

// loadSettings resolves config, discarding load warnings the way most
// callers do.
func loadSettings() Settings {
	s, _ := Load()
	return s
}

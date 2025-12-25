// Package config handles configuration loading for bumper-lanes.
// Priority: Personal (.git/bumper-config.json) > Repo (.bumper-lanes.json) > Default
package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// DefaultThreshold is the default diff point threshold.
	DefaultThreshold = 400

	// DefaultViewMode is the default visualization mode.
	DefaultViewMode = "tree"

	// ValidModes lists all valid visualization modes.
	// This should match git-diff-tree --list-modes output.
	ValidModes = "tree collapsed smart topn icicle brackets"
)

// Config represents bumper-lanes configuration.
type Config struct {
	Threshold          int    `json:"threshold,omitempty"`
	DefaultViewMode    string `json:"default_view_mode,omitempty"`
	StatusLinePrompted bool   `json:"status_line_prompted,omitempty"`
}

// GetGitDir returns the absolute git directory path.
func GetGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--absolute-git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitDir is an alias for internal use (backwards compat).
func getGitDir() (string, error) {
	return GetGitDir()
}

// getRepoRoot returns the repository root path.
func getRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// loadConfigFile reads and parses a JSON config file.
func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadThreshold returns the configured threshold value.
// Priority: Personal > Repo > Default
func LoadThreshold() int {
	gitDir, err := getGitDir()
	if err != nil {
		return DefaultThreshold
	}
	repoRoot, err := getRepoRoot()
	if err != nil {
		return DefaultThreshold
	}

	// Priority 1: Personal config (untracked, in .git dir)
	personalPath := filepath.Join(gitDir, "bumper-config.json")
	if cfg, err := loadConfigFile(personalPath); err == nil && cfg.Threshold > 0 {
		return cfg.Threshold
	}

	// Priority 2: Repo config (tracked, in repo root)
	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	if cfg, err := loadConfigFile(repoPath); err == nil && cfg.Threshold > 0 {
		return cfg.Threshold
	}

	// Priority 3: Default
	return DefaultThreshold
}

// LoadViewMode returns the configured default view mode.
// Priority: Personal > Repo > Default
func LoadViewMode() string {
	gitDir, err := getGitDir()
	if err != nil {
		return DefaultViewMode
	}
	repoRoot, err := getRepoRoot()
	if err != nil {
		return DefaultViewMode
	}

	// Priority 1: Personal config
	personalPath := filepath.Join(gitDir, "bumper-config.json")
	if cfg, err := loadConfigFile(personalPath); err == nil && cfg.DefaultViewMode != "" {
		if isValidMode(cfg.DefaultViewMode) {
			return cfg.DefaultViewMode
		}
	}

	// Priority 2: Repo config
	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	if cfg, err := loadConfigFile(repoPath); err == nil && cfg.DefaultViewMode != "" {
		if isValidMode(cfg.DefaultViewMode) {
			return cfg.DefaultViewMode
		}
	}

	// Priority 3: Default
	return DefaultViewMode
}

// isValidMode checks if the mode is in the valid modes list.
func isValidMode(mode string) bool {
	for _, valid := range strings.Fields(ValidModes) {
		if mode == valid {
			return true
		}
	}
	return false
}

// SaveRepoConfig writes threshold to repo config file.
func SaveRepoConfig(threshold int) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	cfg := Config{Threshold: threshold}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(repoRoot, ".bumper-lanes.json")
	return os.WriteFile(path, data, 0644)
}

// SavePersonalConfig writes threshold to personal config file.
func SavePersonalConfig(threshold int) error {
	gitDir, err := getGitDir()
	if err != nil {
		return err
	}

	cfg := Config{Threshold: threshold}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(gitDir, "bumper-config.json")
	return os.WriteFile(path, data, 0644)
}

// LoadStatusLinePrompted checks if user has been prompted about status line.
// This is stored in personal config (untracked).
func LoadStatusLinePrompted() bool {
	gitDir, err := getGitDir()
	if err != nil {
		return true // Fail open - don't prompt if we can't check
	}

	personalPath := filepath.Join(gitDir, "bumper-config.json")
	if cfg, err := loadConfigFile(personalPath); err == nil {
		return cfg.StatusLinePrompted
	}
	return false
}

// SaveStatusLinePrompted marks that user has been prompted.
// Preserves existing config values.
func SaveStatusLinePrompted() error {
	gitDir, err := getGitDir()
	if err != nil {
		return err
	}

	personalPath := filepath.Join(gitDir, "bumper-config.json")

	// Load existing config or create new
	cfg, _ := loadConfigFile(personalPath)
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.StatusLinePrompted = true

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(personalPath, data, 0644)
}

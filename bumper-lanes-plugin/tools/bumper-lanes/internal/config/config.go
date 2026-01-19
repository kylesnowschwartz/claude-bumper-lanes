// Package config handles configuration loading for bumper-lanes.
// Config files (in precedence order):
//  1. .bumper-lanes.json at repo root (highest priority)
//  2. ~/.config/bumper-lanes/config.json (global fallback)
//  3. Built-in defaults (lowest priority)
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
	DefaultThreshold = 600

	// DefaultViewMode is the default visualization mode.
	DefaultViewMode = "tree"

	// ValidModes lists all valid visualization modes.
	// This should match diff-viz v2.4.0 render.ValidModes.
	ValidModes = "tree smart sparkline-tree hotpath icicle brackets gauge depth stat"
)

// Config represents bumper-lanes configuration.
// Threshold: nil=use default (600), 0=disabled, 50-2000=active threshold
// ShowDiffViz: nil=default (true), false=hide diff visualization
type Config struct {
	Threshold       *int   `json:"threshold,omitempty"`
	DefaultViewMode string `json:"default_view_mode,omitempty"`
	DefaultViewOpts string `json:"default_view_opts,omitempty"` // e.g., "--width 80 --depth 3"
	ShowDiffViz     *bool  `json:"show_diff_viz,omitempty"`
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

// getGlobalConfigPath returns the path to the global config file.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config/bumper-lanes/config.json.
func getGlobalConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "bumper-lanes", "config.json")
}

// loadMergedConfig loads config from global and repo locations, merging them.
// Repo config values override global config values.
// Returns an empty Config if neither file exists (never nil).
func loadMergedConfig() *Config {
	merged := &Config{}

	// Load global config first (lower priority)
	if globalPath := getGlobalConfigPath(); globalPath != "" {
		if global, err := loadConfigFile(globalPath); err == nil {
			merged = global
		}
	}

	// Load repo config and override (higher priority)
	repoRoot, err := getRepoRoot()
	if err != nil {
		return merged
	}
	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	repo, err := loadConfigFile(repoPath)
	if err != nil {
		return merged
	}

	// Merge: repo values override global (non-nil pointers and non-empty strings)
	if repo.Threshold != nil {
		merged.Threshold = repo.Threshold
	}
	if repo.DefaultViewMode != "" {
		merged.DefaultViewMode = repo.DefaultViewMode
	}
	if repo.DefaultViewOpts != "" {
		merged.DefaultViewOpts = repo.DefaultViewOpts
	}
	if repo.ShowDiffViz != nil {
		merged.ShowDiffViz = repo.ShowDiffViz
	}

	return merged
}

// LoadThreshold returns the configured threshold value.
// Checks repo config first, then global config, then returns DefaultThreshold.
// Returns 0 if explicitly disabled.
func LoadThreshold() int {
	cfg := loadMergedConfig()
	if cfg.Threshold != nil {
		return *cfg.Threshold
	}
	return DefaultThreshold
}

// IsDisabled returns true if the given threshold means enforcement is disabled.
func IsDisabled(threshold int) bool {
	return threshold == 0
}

// LoadViewMode returns the configured default view mode.
// Checks repo config first, then global config, then returns DefaultViewMode.
func LoadViewMode() string {
	cfg := loadMergedConfig()
	if cfg.DefaultViewMode != "" && isValidMode(cfg.DefaultViewMode) {
		return cfg.DefaultViewMode
	}
	return DefaultViewMode
}

// LoadViewOpts returns the configured default view options (e.g., "--width 80").
// Checks repo config first, then global config.
func LoadViewOpts() string {
	cfg := loadMergedConfig()
	return cfg.DefaultViewOpts
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

// LoadShowDiffViz returns whether diff visualization should be shown.
// Checks repo config first, then global config, then returns true (default).
func LoadShowDiffViz() bool {
	cfg := loadMergedConfig()
	if cfg.ShowDiffViz != nil {
		return *cfg.ShowDiffViz
	}
	return true
}

// GetConfigPath returns the path to .bumper-lanes.json (or empty if not in a repo).
func GetConfigPath() string {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return ""
	}
	return filepath.Join(repoRoot, ".bumper-lanes.json")
}

// GetGlobalConfigPath returns the path to the global config file.
// Exported for documentation and debugging.
func GetGlobalConfigPath() string {
	return getGlobalConfigPath()
}

// SaveRepoConfig writes threshold to repo config file.
func SaveRepoConfig(threshold int) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	cfg := Config{Threshold: &threshold}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(repoRoot, ".bumper-lanes.json")
	return os.WriteFile(path, data, 0644)
}

// SaveConfig writes the full config to .bumper-lanes.json, preserving existing values.
func SaveConfig(updates Config) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	path := filepath.Join(repoRoot, ".bumper-lanes.json")

	// Load existing config to preserve other fields
	existing, _ := loadConfigFile(path)
	if existing == nil {
		existing = &Config{}
	}

	// Apply updates (non-nil pointers and non-empty strings override)
	if updates.Threshold != nil {
		existing.Threshold = updates.Threshold
	}
	if updates.DefaultViewMode != "" {
		existing.DefaultViewMode = updates.DefaultViewMode
	}
	if updates.DefaultViewOpts != "" {
		existing.DefaultViewOpts = updates.DefaultViewOpts
	}
	if updates.ShowDiffViz != nil {
		existing.ShowDiffViz = updates.ShowDiffViz
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

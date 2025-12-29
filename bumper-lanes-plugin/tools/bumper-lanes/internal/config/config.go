// Package config handles configuration loading for bumper-lanes.
// Config file: .bumper-lanes.json at repo root (users can gitignore if desired)
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
	// This should match diff-viz v2.0.0 render.ValidModes.
	ValidModes = "tree smart sparkline-tree hotpath icicle brackets gauge depth heatmap stat"
)

// Config represents bumper-lanes configuration.
// Threshold: nil=use default (400), 0=disabled, 50-2000=active threshold
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

// LoadThreshold returns the configured threshold value.
// Returns 0 if explicitly disabled, DefaultThreshold if not set.
func LoadThreshold() int {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return DefaultThreshold
	}

	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	cfg, err := loadConfigFile(repoPath)
	if err != nil {
		return DefaultThreshold
	}

	// nil = not set (use default), non-nil = explicit value (including 0 for disabled)
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
func LoadViewMode() string {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return DefaultViewMode
	}

	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	if cfg, err := loadConfigFile(repoPath); err == nil && cfg.DefaultViewMode != "" {
		if isValidMode(cfg.DefaultViewMode) {
			return cfg.DefaultViewMode
		}
	}

	return DefaultViewMode
}

// LoadViewOpts returns the configured default view options (e.g., "--width 80").
func LoadViewOpts() string {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return ""
	}

	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	if cfg, err := loadConfigFile(repoPath); err == nil && cfg.DefaultViewOpts != "" {
		return cfg.DefaultViewOpts
	}

	return ""
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
// Returns true (default) if not configured, false if explicitly disabled.
func LoadShowDiffViz() bool {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return true // Default to showing
	}

	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	cfg, err := loadConfigFile(repoPath)
	if err != nil {
		return true
	}

	if cfg.ShowDiffViz != nil {
		return *cfg.ShowDiffViz
	}

	return true // Default to showing
}

// GetConfigPath returns the path to .bumper-lanes.json (or empty if not in a repo).
func GetConfigPath() string {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return ""
	}
	return filepath.Join(repoRoot, ".bumper-lanes.json")
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

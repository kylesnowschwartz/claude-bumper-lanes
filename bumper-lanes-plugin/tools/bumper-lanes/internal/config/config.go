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
type Config struct {
	Threshold       int    `json:"threshold,omitempty"`
	DefaultViewMode string `json:"default_view_mode,omitempty"`
	DefaultViewOpts string `json:"default_view_opts,omitempty"` // e.g., "--width 80 --depth 3"
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
func LoadThreshold() int {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return DefaultThreshold
	}

	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	if cfg, err := loadConfigFile(repoPath); err == nil && cfg.Threshold > 0 {
		return cfg.Threshold
	}

	return DefaultThreshold
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

	cfg := Config{Threshold: threshold}
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

	// Apply updates (non-zero values override)
	if updates.Threshold > 0 {
		existing.Threshold = updates.Threshold
	}
	if updates.DefaultViewMode != "" {
		existing.DefaultViewMode = updates.DefaultViewMode
	}
	if updates.DefaultViewOpts != "" {
		existing.DefaultViewOpts = updates.DefaultViewOpts
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

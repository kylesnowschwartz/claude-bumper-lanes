package hooks

import (
	"fmt"
	"os"
	"strconv"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
)

// ConfigShow displays the current threshold configuration.
func ConfigShow() error {
	cfg := loadConfig(logging.New(os.Getenv("CLAUDE_CODE_SESSION_ID"), "config_cmd"))
	threshold := cfg.Threshold

	fmt.Printf("Threshold: %d points", threshold)
	if config.IsDisabled(threshold) {
		fmt.Print(" (disabled)")
	}
	fmt.Println()

	// Show source with helpful paths
	repoPath := config.GetConfigPath()
	globalPath := config.GetGlobalConfigPath()

	repoExists := fileExists(repoPath)
	globalExists := fileExists(globalPath)

	fmt.Println()
	if repoExists && globalExists {
		fmt.Printf("Config: %s (repo, overrides global)\n", repoPath)
		fmt.Printf("Global: %s\n", globalPath)
	} else if repoExists {
		fmt.Printf("Config: %s (repo)\n", repoPath)
		fmt.Printf("Global: %s (not found)\n", globalPath)
	} else if globalExists {
		fmt.Printf("Config: %s (global)\n", globalPath)
		fmt.Printf("Repo:   (no .bumper-lanes.json)\n")
	} else {
		fmt.Println("Config: (using defaults)")
		fmt.Printf("Global: %s (create for viz-only mode)\n", globalPath)
	}
	if config.HasPluginOptions() {
		fmt.Println("Plugin: values set via /plugin > claude-bumper-lanes (repo file still overrides)")
	}
	if globalExists {
		fmt.Printf("Note: the global file is deprecated - move these values to the plugin's settings (/plugin > claude-bumper-lanes). File support ends in v5.\n")
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ConfigSet saves threshold to config (.bumper-lanes.json).
func ConfigSet(value string) error {
	threshold, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid threshold value: %s", value)
	}

	if threshold < 50 || threshold > 2000 {
		return fmt.Errorf("threshold must be between 50 and 2000")
	}

	if err := config.SaveRepoConfig(threshold); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Threshold set to %d (saved to .bumper-lanes.json)\n", threshold)
	return nil
}

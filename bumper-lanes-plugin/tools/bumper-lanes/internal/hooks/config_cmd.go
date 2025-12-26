package hooks

import (
	"fmt"
	"strconv"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
)

// ConfigShow displays the current threshold configuration.
func ConfigShow() error {
	threshold := config.LoadThreshold()
	viewMode := config.LoadViewMode()

	fmt.Printf("Threshold: %d points\n", threshold)
	fmt.Printf("Default view mode: %s\n", viewMode)

	// Show source
	if threshold == config.DefaultThreshold {
		fmt.Println("Source: default")
	} else {
		fmt.Println("Source: config file")
	}

	return nil
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

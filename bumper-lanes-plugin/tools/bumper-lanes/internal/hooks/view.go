package hooks

import (
	"fmt"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/diff-viz/v2/render"
)

// ViewShow displays the current view mode and available options.
func ViewShow(sessionID string) error {
	validModes := getValidModes()

	// Try to get current mode from session
	currentMode := ""
	sess, err := state.Load(sessionID)
	if err == nil {
		currentMode = sess.GetViewMode()
	}

	// Fall back to config default
	configMode := config.LoadViewMode()
	if currentMode == "" {
		currentMode = configMode
	}
	if currentMode == "" {
		currentMode = "tree" // Ultimate fallback
	}

	fmt.Printf("Current mode: %s\n", currentMode)
	if configMode != "" && configMode != currentMode {
		fmt.Printf("Config default: %s\n", configMode)
	}
	fmt.Printf("Available modes: %s\n", strings.Join(validModes, ", "))
	return nil
}

// View handles the view user command.
// It sets the visualization mode for both session state and project config.
// opts contains additional flags like "--width 100 --depth 3".
func View(sessionID, mode, opts string) error {
	sess, err := state.Load(sessionID)
	if err != nil {
		return fmt.Errorf("no session state for %s", sessionID)
	}

	// Validate mode
	validModes := getValidModes()
	if !isValidMode(mode, validModes) {
		return fmt.Errorf("invalid mode %q. Valid: %s", mode, strings.Join(validModes, ", "))
	}

	// Update session state (immediate effect)
	sess.SetViewMode(mode)
	sess.SetViewOpts(opts)
	if err := sess.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Persist to project config (.bumper-lanes.json) for future sessions
	cfg := config.Config{DefaultViewMode: mode, DefaultViewOpts: opts}
	if err := config.SaveConfig(cfg); err != nil {
		if opts != "" {
			fmt.Printf("View mode set to: %s %s (session only - config save failed: %v)\n", mode, opts, err)
		} else {
			fmt.Printf("View mode set to: %s (session only - config save failed: %v)\n", mode, err)
		}
		return nil
	}

	if opts != "" {
		fmt.Printf("View mode set to: %s %s\n", mode, opts)
	} else {
		fmt.Printf("View mode set to: %s\n", mode)
	}
	return nil
}

// getValidModes returns the list of supported visualization modes from diff-viz.
func getValidModes() []string {
	return render.ValidModes
}

// isValidMode checks if mode is valid using diff-viz's exported validation.
func isValidMode(mode string, _ []string) bool {
	return render.IsValidMode(mode)
}

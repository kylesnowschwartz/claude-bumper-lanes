package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// View handles the view user command.
// It sets the visualization mode for both session and personal config.
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

	// Persist to personal config (.git/bumper-config.json) for future sessions
	if err := persistViewModeToConfig(mode); err != nil {
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

// persistViewModeToConfig saves the view mode to personal config.
func persistViewModeToConfig(mode string) error {
	gitDir, err := config.GetGitDir()
	if err != nil {
		return err
	}

	personalConfig := filepath.Join(gitDir, "bumper-config.json")

	// Read existing config or create new
	var cfg map[string]interface{}
	data, err := os.ReadFile(personalConfig)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			cfg = make(map[string]interface{})
		}
	} else {
		cfg = make(map[string]interface{})
	}

	// Update view mode
	cfg["default_view_mode"] = mode

	// Write back
	newData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(personalConfig, newData, 0644)
}

// getValidModes queries git-diff-tree for valid modes.
func getValidModes() []string {
	diffTreeBin := GetGitDiffTreePath()
	cmd := exec.Command(diffTreeBin, "--list-modes")
	output, err := cmd.Output()
	if err != nil {
		// Fallback
		return []string{"tree", "collapsed", "smart", "topn", "icicle", "brackets"}
	}
	return strings.Fields(strings.TrimSpace(string(output)))
}

// isValidMode checks if mode is in the list.
func isValidMode(mode string, validModes []string) bool {
	for _, m := range validModes {
		if m == mode {
			return true
		}
	}
	return false
}

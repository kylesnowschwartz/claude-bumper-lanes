package hooks

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// View handles the view user command.
// It sets the visualization mode for the session.
func View(sessionID, mode string) error {
	sess, err := state.Load(sessionID)
	if err != nil {
		return fmt.Errorf("no session state for %s", sessionID)
	}

	// Validate mode
	validModes := getValidModes()
	if !isValidMode(mode, validModes) {
		return fmt.Errorf("invalid mode %q. Valid: %s", mode, strings.Join(validModes, ", "))
	}

	sess.SetViewMode(mode)

	if err := sess.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("View mode set to: %s\n", mode)
	return nil
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

package hooks

import (
	"fmt"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Reset handles the reset user command.
// It captures a new baseline and resets the accumulated score.
func Reset(sessionID string) error {
	// Load session state
	sess, err := state.Load(sessionID)
	if err != nil {
		return fmt.Errorf("no session state for %s", sessionID)
	}

	// Capture new baseline tree
	newTree, err := CaptureTree()
	if err != nil {
		return fmt.Errorf("failed to capture tree: %w", err)
	}

	// Get current branch
	currentBranch := GetCurrentBranch()

	// Reset baseline
	sess.ResetBaseline(newTree, currentBranch)

	// Save state
	if err := sess.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("Baseline reset. New tree: %s\n", newTree[:12])
	return nil
}

package hooks

import (
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// SessionStart handles the SessionStart hook event.
// It captures the baseline tree and initializes session state.
func SessionStart(input *HookInput) error {
	// Check if this is a git repository
	if !IsGitRepo() {
		return nil // Fail open - not a git repo
	}

	// Capture baseline tree
	baselineTree, err := CaptureTree()
	if err != nil {
		return nil // Fail open
	}

	// Get current branch for staleness detection
	baselineBranch := GetCurrentBranch()

	// Load threshold from config
	threshold := config.LoadThreshold()

	// Create and save session state
	sess, err := state.New(input.SessionID, baselineTree, baselineBranch, threshold)
	if err != nil {
		return nil // Fail open
	}

	return sess.Save()
}

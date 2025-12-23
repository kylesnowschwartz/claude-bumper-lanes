package hooks

import (
	"fmt"
	"os"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// PostToolUse handles the PostToolUse hook event for Write/Edit tools.
// It provides fuel gauge warnings based on threshold consumption.
// Returns exit code 2 to ensure stderr reaches Claude.
func PostToolUse(input *HookInput) (exitCode int) {
	// Validate hook event
	if input.HookEventName != "PostToolUse" {
		return 0
	}

	// Only process Write/Edit tools
	switch input.ToolName {
	case "Write", "Edit":
		// Proceed
	default:
		return 0
	}

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		return 0 // Fail open
	}

	// If paused, exit silently
	if sess.Paused {
		return 0
	}

	// Capture current tree
	currentTree, err := CaptureTree()
	if err != nil {
		return 0
	}

	// Get diff stats
	stats := getStatsJSON(sess.PreviousTree, currentTree)
	if stats == nil {
		return 0
	}

	// Calculate score
	score := scoring.Calculate(stats)
	newAccum := sess.AccumulatedScore + score.Score

	// Update incremental state
	sess.UpdateIncremental(currentTree, newAccum)
	sess.Save()

	// Calculate percentage
	pct := (newAccum * 100) / sess.ThresholdLimit

	// Output fuel gauge to stderr based on threshold tier
	// Exit 2 ensures stderr reaches Claude (per docs)
	if pct >= 90 {
		fmt.Fprintf(os.Stderr, "CRITICAL: Review budget near critical (%d%%). %d/%d pts. STOP accepting work. Inform user checkpoint needed NOW.\n", pct, newAccum, sess.ThresholdLimit)
		return 2
	} else if pct >= 75 {
		fmt.Fprintf(os.Stderr, "WARNING: Review budget at %d%% (%d/%d pts). Complete current work, then ask user about checkpoint.\n", pct, newAccum, sess.ThresholdLimit)
		return 2
	} else if pct >= 50 {
		fmt.Fprintf(os.Stderr, "NOTICE: %d%% budget used (%d/%d pts). Wrap up current task soon.\n", pct, newAccum, sess.ThresholdLimit)
		return 2
	}

	// Under 50% - silent
	return 0
}

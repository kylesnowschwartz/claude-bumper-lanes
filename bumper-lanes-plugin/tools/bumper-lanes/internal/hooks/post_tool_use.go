package hooks

import (
	"fmt"
	"regexp"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// gitCommitPattern matches git commit commands with optional flags.
// Matches: git commit, git -C /path commit, git --git-dir=/x commit
// Rejects: prose like "use git to commit"
var gitCommitPattern = regexp.MustCompile(`git\s+(-{1,2}[A-Za-z-]+([ =]("[^"]*"|\S+))?\s+)*commit\b`)

// PostToolUse handles the PostToolUse hook event.
// For Write/Edit: provides fuel gauge warnings
// For Bash: detects git commits and auto-resets baseline
// Feedback reaches Claude via hookSpecificOutput.additionalContext (exit 0).
func PostToolUse(input *HookInput) (exitCode int) {
	// Validate hook event
	if input.HookEventName != "PostToolUse" {
		return 0
	}

	// Route based on tool type
	switch input.ToolName {
	case "Write", "Edit":
		return handleWriteEdit(input)
	case "Bash":
		return handleBashCommit(input)
	default:
		return 0
	}
}

// handleBashCommit detects git commits and auto-resets baseline.
func handleBashCommit(input *HookInput) int {
	log := logging.New(input.SessionID, "post_tool_use")

	// Need command to check
	if input.ToolInput == nil || input.ToolInput.Command == "" {
		return 0
	}

	// Check if this is a git commit command
	if !gitCommitPattern.MatchString(input.ToolInput.Command) {
		return 0
	}

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		log.Warn("failed to load session (bash commit): %v (failing open)", err)
		return 0 // No session - fail open
	}

	// Capture current tree including untracked files
	// Must use CaptureTree() (same as manual reset) so pre-existing
	// untracked files are included in baseline and don't get re-counted
	currentTree, err := CaptureTree()
	if err != nil {
		log.Warn("failed to capture tree after commit: %v (failing open)", err)
		return 0 // Failed to capture tree - fail open
	}

	// Reset baseline
	currentBranch := GetCurrentBranch()
	sess.ResetBaseline(currentTree, currentBranch)
	if err := sess.Save(); err != nil {
		return 0
	}

	// Output feedback
	threshold := config.LoadThreshold()
	WriteContext("PostToolUse", fmt.Sprintf("bumper-lanes: auto-reset after commit. Fresh budget: %d pts.", threshold))
	return 0
}

// handleWriteEdit provides fuel gauge warnings after file modifications.
func handleWriteEdit(input *HookInput) int {
	log := logging.New(input.SessionID, "post_tool_use")

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		log.Warn("failed to load session (write/edit): %v (failing open)", err)
		return 0 // Fail open
	}

	// If paused, exit silently
	if sess.Paused {
		return 0
	}

	// If threshold is 0 (disabled), exit silently (no fuel gauge)
	if sess.ThresholdLimit == 0 {
		return 0
	}

	// Get diff stats from baseline (fresh calculation, not incremental)
	// This allows score to decrease when user manually deletes/reverts changes
	stats := getStatsJSON(sess.BaselineTree)
	if stats == nil {
		return 0
	}

	// Calculate fresh score from baseline
	result := scoring.Calculate(stats)
	freshScore := result.Score

	// Update state with fresh score
	sess.SetScore(freshScore)
	sess.Save()

	// Calculate percentage
	pct := (freshScore * 100) / sess.ThresholdLimit

	// Fuel gauge tiers (70%, 90%) reach Claude via additionalContext
	if pct >= 90 {
		WriteContext("PostToolUse", fmt.Sprintf("bumper-lanes: review budget at %d%% (%d/%d pts). Finish the current increment and pause for review - do not start new work.", pct, freshScore, sess.ThresholdLimit))
		return 0
	} else if pct >= 70 {
		WriteContext("PostToolUse", fmt.Sprintf("bumper-lanes: review budget at %d%% (%d/%d pts). Plan to wrap up the current increment soon.", pct, freshScore, sess.ThresholdLimit))
		return 0
	}

	// Under 70% - silent
	return 0
}

package hooks

import (
	"fmt"
	"os"
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
// Returns exit code 2 to ensure stderr reaches Claude.
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
	fmt.Fprintf(os.Stderr, "✓ Bumper lanes: Auto-reset after commit. Fresh budget: %d pts.\n", threshold)
	return 2
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

	// Auto-reset if working tree is clean (no uncommitted changes)
	// Detects: external commits, IDE commits, manual reverts, git reset
	// Check: current working tree == HEAD (not baseline)
	currentTree, err := CaptureTree()
	if err == nil {
		headTree := GetHeadTree()
		if headTree != "" && currentTree == headTree {
			// Working tree matches HEAD - no uncommitted changes
			currentBranch := GetCurrentBranch()
			sess.ResetBaseline(currentTree, currentBranch)
			sess.Save()

			fmt.Fprintf(os.Stderr, "✓ Bumper lanes: Auto-reset (no uncommitted changes). Fresh budget: %d pts.\n", sess.ThresholdLimit)
			return 2
		}
		// If trees don't match or HEAD unavailable, fall through (working tree is dirty)
	}

	// Update state with fresh score
	sess.SetScore(freshScore)
	sess.Save()

	// Calculate percentage
	pct := (freshScore * 100) / sess.ThresholdLimit

	// Output fuel gauge to stderr based on threshold tier
	// Exit 2 ensures stderr reaches Claude (per docs)
	// Tiers: 70% NOTICE, 90% WARNING
	if pct >= 90 {
		fmt.Fprintf(os.Stderr, "WARNING: Review budget at %d%% (%d/%d pts). Complete current work, then ask user about checkpoint.\n", pct, freshScore, sess.ThresholdLimit)
		return 2
	} else if pct >= 70 {
		fmt.Fprintf(os.Stderr, "NOTICE: %d%% budget used (%d/%d pts). Wrap up current task soon.\n", pct, freshScore, sess.ThresholdLimit)
		return 2
	}

	// Under 70% - silent
	return 0
}

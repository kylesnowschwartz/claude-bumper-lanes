package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
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
		return 0 // No session - fail open
	}

	// Get the tree SHA from HEAD (what was just committed)
	cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
	output, err := cmd.Output()
	if err != nil {
		return 0 // Failed to get tree - fail open
	}
	currentTree := strings.TrimSpace(string(output))

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

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		return 0 // Fail open
	}

	// If paused, exit silently
	if sess.Paused {
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

	// Output fuel gauge to stderr based on threshold tier
	// Exit 2 ensures stderr reaches Claude (per docs)
	if pct >= 90 {
		fmt.Fprintf(os.Stderr, "CRITICAL: Review budget near critical (%d%%). %d/%d pts. STOP accepting work. Inform user checkpoint needed NOW.\n", pct, freshScore, sess.ThresholdLimit)
		return 2
	} else if pct >= 75 {
		fmt.Fprintf(os.Stderr, "WARNING: Review budget at %d%% (%d/%d pts). Complete current work, then ask user about checkpoint.\n", pct, freshScore, sess.ThresholdLimit)
		return 2
	} else if pct >= 50 {
		fmt.Fprintf(os.Stderr, "NOTICE: %d%% budget used (%d/%d pts). Wrap up current task soon.\n", pct, freshScore, sess.ThresholdLimit)
		return 2
	}

	// Under 50% - silent
	return 0
}

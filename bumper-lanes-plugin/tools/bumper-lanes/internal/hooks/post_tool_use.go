package hooks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
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
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		return handleWriteEdit(input)
	case "Bash":
		return handleBashCommit(input)
	default:
		return 0
	}
}

// noVerifyPattern matches hook-bypassing flags: --no-verify or the short
// form -n (alone or bundled, e.g. -an). Applied only to the git commit
// segment of the command (see commitSegment), not the whole pipeline.
var noVerifyPattern = regexp.MustCompile(`--no-verify\b|\s-[a-zA-Z]*n[a-zA-Z]*\b`)

// commitSegment returns the shell segment containing the git commit
// invocation: from the match to the next command separator. This keeps
// no-verify detection from misreading flags of other commands in a
// pipeline (e.g. `git commit -m x && git log -n 1`).
func commitSegment(command string) string {
	loc := gitCommitPattern.FindStringIndex(command)
	if loc == nil {
		return ""
	}
	segment := command[loc[0]:]
	for _, sep := range []string{"&&", "||", ";", "|", "\n"} {
		if i := strings.Index(segment, sep); i >= 0 {
			segment = segment[:i]
		}
	}
	return segment
}

// handleBashCommit detects git commits and auto-resets baseline.
// The evidence that a commit landed is HEAD moving between PreToolUse
// (which recorded it in HeadBeforeCommit) and now - a rejected or no-op
// commit leaves HEAD in place, and a quiet one still moves it.
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

	// Consume the pending-commit record regardless of outcome.
	headBefore := sess.HeadBeforeCommit
	if headBefore != "" {
		sess.HeadBeforeCommit = ""
		sess.Save()
	}

	if headBefore == "" || GetHeadCommit() == headBefore {
		log.Info("HEAD did not move - commit did not land, baseline not reset")
		return 0
	}

	// Apply the reset policy now that the commit is proven.
	policy := config.LoadResetOn()
	switch policy {
	case config.ResetOnHuman:
		log.Info("reset_on=human - commit does not reset baseline")
		return 0
	case config.ResetOnVerifiedCommit:
		if noVerifyPattern.MatchString(commitSegment(input.ToolInput.Command)) {
			log.Info("reset_on=verified-commit and commit bypasses hooks - baseline not reset")
			WriteContext("PostToolUse", "bumper-lanes: commit used --no-verify, so the review budget was NOT reset (reset_on=verified-commit). Commit with hooks enabled to reset the budget.")
			return 0
		}
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
	scoreAtReset := sess.Score
	currentBranch := GetCurrentBranch()
	sess.ResetBaseline(currentTree, currentBranch)
	if err := sess.Save(); err != nil {
		return 0
	}

	cause := events.CauseCommit
	if policy == config.ResetOnVerifiedCommit {
		cause = events.CauseVerifiedCommit
	}
	if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.Reset, Score: scoreAtReset, Limit: sess.ThresholdLimit, Cause: cause}); err != nil {
		log.Warn("failed to append event: %v", err)
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

	// Tripwires fire at any score: small edits in high-risk classes
	// (CI, dependencies, harness config, test skips) get named immediately.
	freshTripwires := sess.AddTripwires(detectTripwires(stats, sess.BaselineTree))

	// Update state with fresh score (and any new tripwires)
	sess.SetScore(freshScore)
	sess.NetLines = result.NetLines
	sess.Save()

	var messages []string
	for _, hit := range freshTripwires {
		if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.Tripwire, Score: freshScore, Limit: sess.ThresholdLimit, Tripwire: hit}); err != nil {
			log.Warn("failed to append event: %v", err)
		}
	}
	if len(freshTripwires) > 0 {
		messages = append(messages, fmt.Sprintf("bumper-lanes TRIPWIRE: %s - high-risk change class. Point this out to the user for review regardless of budget.", strings.Join(freshTripwires, ", ")))
	}

	// Fuel gauge tiers (70%, 90%) reach Claude via additionalContext
	pct := (freshScore * 100) / sess.ThresholdLimit
	if pct >= 90 {
		messages = append(messages, fmt.Sprintf("bumper-lanes: %s. Finish the current increment and pause for review; do not start new work.", budgetLine(freshScore, sess.ThresholdLimit)))
	} else if pct >= 70 {
		messages = append(messages, fmt.Sprintf("bumper-lanes: %s. Fit the rest of this increment in the remaining budget; defer anything new.", budgetLine(freshScore, sess.ThresholdLimit)))
	}

	if len(messages) > 0 {
		WriteContext("PostToolUse", strings.Join(messages, "\n"))
	}
	return 0
}

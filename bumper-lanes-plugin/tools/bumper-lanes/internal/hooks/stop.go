package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Stop handles the Stop hook event.
// It checks if the diff threshold is exceeded and notifies Claude if so.
//
// IMPORTANT: Claude Code Stop Hook Semantics (confusing but documented)
//
// The Stop hook fires when Claude tries to finish a turn. The response JSON has:
//
//   - "continue": Controls whether Claude keeps working after the hook
//
//   - true:  Claude can continue (talk, read files, use tools)
//
//   - false: Claude stops entirely (can't even explain what happened)
//
//   - "decision": Only meaningful for Stop hooks, controls stopping behavior
//
//   - "block": Prevents Claude from STOPPING (counterintuitively keeps Claude working)
//
//   - omitted: Normal behavior
//
// The naming is confusing because "block" doesn't block Claude - it blocks the STOP.
// Per Claude Code docs: "continue: false takes precedence over decision: block"
//
// For bumper-lanes threshold enforcement:
//   - We use continue: true so Claude can still communicate with the user,
//     read files to help with review, and explain the threshold situation.
//   - We use decision: "block" + reason to show the threshold message.
//   - Actual write/edit prevention is done via fuel gauge warnings that guide
//     Claude's behavior, not by hard-blocking at the Stop hook level.
//   - This is "soft enforcement" - Claude sees the warning and should stop
//     accepting new work, but can still help the user review changes.
//
// Reference: https://docs.anthropic.com/en/docs/claude-code/hooks
func Stop(input *HookInput) error {
	// Check if this is a git repository
	if !IsGitRepo() {
		return nil
	}

	// Acquire lock to prevent parallel Stop hooks from racing
	lockDir, err := acquireLock(input.SessionID)
	if err != nil {
		return nil // Another instance has the lock
	}
	defer releaseLock(lockDir)

	// If already blocked once, allow stop to prevent infinite loop
	if input.StopHookActive {
		return nil
	}

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bumper-lanes: warning: failed to load session state: %v (failing open)\n", err)
		return nil // No baseline - fail open
	}

	// If already triggered, allow stop (PreToolUse is blocking)
	if sess.StopTriggered {
		return nil
	}

	// If paused, track changes but don't enforce
	if sess.Paused {
		// Use fresh score from baseline (not incremental accumulation)
		stats := getStatsJSON(sess.BaselineTree)
		if stats != nil {
			result := scoring.Calculate(stats)
			sess.SetScore(result.Score)
			sess.Save()
		}
		return nil
	}

	// Capture current working tree
	currentTree, err := CaptureTree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bumper-lanes: warning: failed to capture current tree: %v (failing open)\n", err)
		return nil // Fail open
	}

	// Detect branch switch - auto-reset baseline
	currentBranch := GetCurrentBranch()
	if sess.BaselineBranch != "" && currentBranch != "" && sess.BaselineBranch != currentBranch {
		sess.ResetBaseline(currentTree, currentBranch)
		sess.Save()

		// Output branch switch message
		resp := StopResponse{
			Continue:       true,
			SystemMessage:  fmt.Sprintf("↪ Bumper lanes: Branch changed (%s → %s) — baseline auto-reset.", sess.BaselineBranch, currentBranch),
			SuppressOutput: false,
		}
		return WriteResponse(resp)
	}

	// Get diff stats from baseline (fresh calculation, not incremental)
	// This allows score to decrease when user manually deletes/reverts changes
	stats := getStatsJSON(sess.BaselineTree)
	if stats == nil {
		fmt.Fprintf(os.Stderr, "bumper-lanes: warning: failed to get diff stats (failing open)\n")
		return nil // Fail open
	}

	// Calculate fresh score from baseline
	result := scoring.Calculate(stats)
	freshScore := result.Score

	// Check threshold
	if freshScore <= sess.ThresholdLimit {
		// Under threshold - update state and allow
		sess.SetScore(freshScore)
		sess.Save()
		return nil
	}

	// Over threshold - set stop_triggered and block
	sess.SetStopTriggered(true)
	sess.SetScore(freshScore)
	sess.Save()

	// Format breakdown message (stats are already from baseline)
	pct := (freshScore * 100) / sess.ThresholdLimit
	reason := fmt.Sprintf(`

⚠️  Bumper lanes: Diff threshold exceeded

Score: %d / %d points (%d%%)
- New file additions: %d lines (1.0×)
- Edit additions: %d lines (1.3×)
- Files touched: %d
- Scatter penalty: %d pts

Ask the User: Would you like to conduct a structured, manual review?

This workflow ensures incremental code review at predictable checkpoints.

`, freshScore, sess.ThresholdLimit, pct, result.NewAdditions, result.EditAdditions, result.FilesTouched, result.ScatterPenalty)

	// Build response - see function doc comment for explanation of these confusing semantics
	resp := StopResponse{
		// continue: true = Claude can keep working (talk, read, help with review)
		// continue: false would prevent Claude from even explaining what happened
		Continue: true,
		// SystemMessage appears in Claude's context
		SystemMessage: "/bumper-reset after code review.",
		// SuppressOutput hides Claude's pending output (the turn it was about to finish)
		SuppressOutput: true,
		// decision: "block" = block the STOP, not block Claude (confusing naming!)
		// This keeps Claude working so it can show the Reason message
		Decision: "block",
		// Reason is shown to the user explaining why we blocked the stop
		Reason: reason,
		ThresholdData: map[string]interface{}{
			"score":                freshScore,
			"threshold_limit":      sess.ThresholdLimit,
			"threshold_percentage": pct,
			"new_additions":        result.NewAdditions,
			"edit_additions":       result.EditAdditions,
			"files_touched":        result.FilesTouched,
			"scatter_penalty":      result.ScatterPenalty,
		},
	}

	return WriteResponse(resp)
}

// getStatsJSON calls git-diff-tree --stats-json and returns parsed stats.
// Compares baselineTree to current working tree.
func getStatsJSON(baselineTree string) *scoring.StatsJSON {
	diffTreeBin := GetGitDiffTreePath()
	cmd := exec.Command(diffTreeBin, "--stats-json", "--baseline", baselineTree)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var stats scoring.StatsJSON
	if err := json.Unmarshal(output, &stats); err != nil {
		return nil
	}

	return &stats
}

// acquireLock creates a lock directory to prevent parallel hook races.
func acquireLock(sessionID string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--absolute-git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	gitDir := strings.TrimSpace(string(output))

	lockDir := filepath.Join(gitDir, "bumper-checkpoints", fmt.Sprintf("stop-lock-%s.lock", sessionID))
	if err := os.Mkdir(lockDir, 0755); err != nil {
		return "", err // Lock already held
	}
	return lockDir, nil
}

// releaseLock removes the lock directory.
func releaseLock(lockDir string) {
	os.Remove(lockDir)
}

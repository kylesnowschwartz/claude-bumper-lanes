package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Stop handles the Stop hook event.
// It checks if the diff threshold is exceeded and blocks if necessary.
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
		return nil // No baseline - fail open
	}

	// If already triggered, allow stop (PreToolUse is blocking)
	if sess.StopTriggered {
		return nil
	}

	// If paused, track changes but don't enforce
	if sess.Paused {
		currentTree, err := CaptureTree()
		if err == nil {
			stats := getStatsJSON(sess.PreviousTree, currentTree)
			if stats != nil {
				score := scoring.Calculate(stats)
				newAccum := sess.AccumulatedScore + score.Score
				sess.UpdateIncremental(currentTree, newAccum)
				sess.Save()
			}
		}
		return nil
	}

	// Capture current working tree
	currentTree, err := CaptureTree()
	if err != nil {
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

	// Get diff stats from git-diff-tree
	stats := getStatsJSON(sess.PreviousTree, currentTree)
	if stats == nil {
		return nil // Fail open
	}

	// Calculate score
	score := scoring.Calculate(stats)
	newAccum := sess.AccumulatedScore + score.Score

	// Check threshold
	if newAccum <= sess.ThresholdLimit {
		// Under threshold - update state and allow
		sess.UpdateIncremental(currentTree, newAccum)
		sess.Save()
		return nil
	}

	// Over threshold - set stop_triggered and block
	sess.SetStopTriggered(true)
	sess.UpdateIncremental(currentTree, newAccum)
	sess.Save()

	// Format breakdown message
	pct := (newAccum * 100) / sess.ThresholdLimit
	reason := fmt.Sprintf(`

⚠️  Bumper lanes: Diff threshold exceeded

Score: %d / %d points (%d%%)
- New file additions: %d lines (1.0×)
- Edit additions: %d lines (1.3×)
- Files touched: %d
- Scatter penalty: %d pts

Ask the User: Would you like to conduct a structured, manual review?

This workflow ensures incremental code review at predictable checkpoints.

`, newAccum, sess.ThresholdLimit, pct, score.NewAdditions, score.EditAdditions, score.FilesTouched, score.ScatterPenalty)

	resp := StopResponse{
		Continue:       true,
		SystemMessage:  "/bumper-reset after code review.",
		SuppressOutput: true,
		Decision:       "block",
		Reason:         reason,
		ThresholdData: map[string]interface{}{
			"score":                newAccum,
			"threshold_limit":      sess.ThresholdLimit,
			"threshold_percentage": pct,
			"new_additions":        score.NewAdditions,
			"edit_additions":       score.EditAdditions,
			"files_touched":        score.FilesTouched,
			"scatter_penalty":      score.ScatterPenalty,
		},
	}

	return WriteResponse(resp)
}

// getStatsJSON calls git-diff-tree --stats-json and returns parsed stats.
func getStatsJSON(baselineTree, currentTree string) *scoring.StatsJSON {
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

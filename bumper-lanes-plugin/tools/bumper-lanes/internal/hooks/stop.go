package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/diff-viz/v2/diff"
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
	// Initialize logger for this session
	log := logging.New(input.SessionID, "stop")

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
		log.Warn("failed to load session state: %v (failing open)", err)
		return nil // No baseline - fail open
	}

	// Always recalculate score to enable bidirectional state transitions.
	// If paused, track changes but don't enforce
	if sess.Paused {
		// Use fresh score from baseline (not incremental accumulation)
		stats := getStatsJSON(sess.BaselineTree)
		if stats != nil {
			result := scoring.Calculate(stats)
			sess.SetScore(result.Score)
			sess.NetLines = result.NetLines
			sess.Save()
		}
		return nil
	}

	// If threshold is 0 (disabled), track changes but don't enforce
	// Same behavior as paused, but config-driven instead of session command
	if sess.ThresholdLimit == 0 {
		stats := getStatsJSON(sess.BaselineTree)
		if stats != nil {
			result := scoring.Calculate(stats)
			sess.SetScore(result.Score)
			sess.NetLines = result.NetLines
			sess.Save()
		}
		return nil
	}

	// Forgive commits that landed since the baseline (pull/rebase/merge)
	// before any score calculation below. After the paused/disabled guards
	// so inert sessions don't pay the git call.
	maybeRebaseBaseline(sess, log)

	// Detect branch switch - auto-reset baseline
	currentBranch := GetCurrentBranch()
	if sess.BaselineBranch != "" && currentBranch != "" && sess.BaselineBranch != currentBranch {
		// Only capture tree when actually needed (branch switch detected)
		// This avoids ~50ms overhead on every Stop invocation
		currentTree, err := CaptureTree()
		if err != nil {
			log.Warn("failed to capture current tree for branch reset: %v (failing open)", err)
			return nil
		}
		scoreAtReset := sess.Score
		sess.ResetBaseline(currentTree, currentBranch, GetHeadCommit())
		sess.Save()
		if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.Reset, Score: scoreAtReset, Limit: sess.ThresholdLimit, Cause: events.CauseBranch}); err != nil {
			log.Warn("failed to append event: %v", err)
		}

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
		log.Warn("failed to get diff stats (failing open)")
		return nil // Fail open
	}

	// Calculate fresh score from baseline
	result := scoring.Calculate(stats)
	freshScore := result.Score

	// Check threshold
	if freshScore <= sess.ThresholdLimit {
		// Under threshold - check if we need to clear StopTriggered flag
		if sess.StopTriggered {
			// Automatic recovery: score dropped below threshold
			sess.SetStopTriggered(false)
			sess.SetScore(freshScore)
			sess.NetLines = result.NetLines
			sess.Save()

			// Notify user of recovery
			pct := 0
			if sess.ThresholdLimit > 0 {
				pct = (freshScore * 100) / sess.ThresholdLimit
			}
			resp := StopResponse{
				Continue:       true,
				SystemMessage:  fmt.Sprintf("✓ Bumper lanes: Auto-recovered (score dropped to %d/%d - %d%%)", freshScore, sess.ThresholdLimit, pct),
				SuppressOutput: false,
			}
			return WriteResponse(resp)
		}

		// Normal case: update state and allow
		sess.SetScore(freshScore)
		sess.NetLines = result.NetLines
		sess.Save()
		return nil
	}

	// Over threshold - set stop_triggered and block
	wasTripped := sess.StopTriggered
	sess.SetStopTriggered(true)
	sess.SetScore(freshScore)
	sess.NetLines = result.NetLines
	// Stamp the effective trip policy for review-clear, which runs from
	// the agent's Bash tool and cannot see plugin userConfig env vars.
	policy := currentReviewPolicy()
	sess.Policy = policy
	sess.Save()
	// Log the trip transition only, not every blocked Stop while tripped.
	if !wasTripped {
		if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.Trip, Score: freshScore, Limit: sess.ThresholdLimit}); err != nil {
			log.Warn("failed to append event: %v", err)
		}
	}

	// The trip packet presents the increment at review altitude: modules,
	// decisions, a scripted next move, and the file-level ground truth.
	// The next move depends on the trip policy: self-review when enabled
	// and available this cycle, otherwise the human packet.
	nextMove := humanNextMove
	if policy.OnTrip == config.OnTripReview {
		switch {
		case policy.MaxAutoReviews >= 0 && sess.AutoReviews >= policy.MaxAutoReviews:
			nextMove = humanNextMove + escalationNote
		case policy.TripwiresBlockAutoReview && len(sess.Tripwires) > 0:
			nextMove = humanNextMove + "\nNote: tripwires fired, so this trip requires the user (tripwires_block_auto_review).\n"
		default:
			nextMove = reviewNextMove(policy.ReviewCommand)
		}
	}
	nextMove += staleBaselineNote(sess)
	reason := buildTripPacket(sess, result, stats, nextMove)
	pct := (freshScore * 100) / sess.ThresholdLimit

	// Desktop notification on the fresh trip only, so an unattended
	// session surfaces the block once instead of on every retried stop.
	notification := ""
	if !wasTripped {
		notification = tripNotification(freshScore, sess.ThresholdLimit)
	}

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
		// OSC 9 desktop notification (empty when this stop was already tripped)
		TerminalSequence: notification,
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

// getStatsJSON uses diff-viz library to get stats from baseline to current tree.
func getStatsJSON(baselineTree string) *diff.StatsJSON {
	// Capture current working tree
	currentTree, err := diff.CaptureCurrentTree()
	if err != nil {
		return nil
	}

	// Get diff stats from baseline to current
	stats, _, err := diff.GetTreeDiffStats(baselineTree, currentTree)
	if err != nil {
		return nil
	}

	jsonStats := stats.ToJSON()
	return &jsonStats
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

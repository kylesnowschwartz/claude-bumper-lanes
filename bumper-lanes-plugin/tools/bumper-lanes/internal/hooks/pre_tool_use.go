package hooks

import (
	"fmt"
	"os"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/enforce"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// PreToolUse handles the PreToolUse hook event.
// It blocks file modification tools (Write, Edit, etc.) when the threshold
// has been exceeded and StopTriggered is true.
//
// NEW (v3.7.0): Before blocking, checks if working tree has become clean
// (matches HEAD) since Stop hook triggered. If clean, auto-resets baseline
// and clears StopTriggered flag, allowing the tool to proceed.
//
// This handles external commits (IDE, terminal) that clean the tree between
// Stop hook firing and the next Write/Edit attempt.
//
// This is the "hard enforcement" layer - it prevents tools from executing
// entirely, complementing the Stop hook which blocks turn completion.
//
// Returns exit code 0 for JSON output (even when blocking).
func PreToolUse(input *hookio.Input) (exitCode int) {
	log := logging.New(input.SessionID, "pre_tool_use")

	// Validate hook event
	if input.HookEventName != "PreToolUse" {
		return 0
	}

	// Only block file modification tools
	switch input.ToolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		// Proceed with threshold check
	case "Bash":
		// Never blocked; only used to record commit evidence.
		recordHeadBeforeCommit(input, log)
		return 0
	default:
		return 0
	}

	// Check if git repo
	if !git.IsRepo() {
		return 0
	}

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		log.Warn("failed to load session: %v (failing open)", err)
		return 0 // Fail open
	}

	// If paused, allow tool
	if sess.Paused {
		return 0
	}

	// If threshold is 0 (disabled), allow tool
	if sess.ThresholdLimit == 0 {
		return 0
	}

	// ╔═══════════════════════════════════════════════════════════╗
	// ║ AUTO-RECOVERY: Recalculate score when StopTriggered      ║
	// ║ This handles external changes that reduce the diff       ║
	// ║ Cost: ~125ms per Write/Edit when blocked (rare)          ║
	// ╚═══════════════════════════════════════════════════════════╝
	// When Stop hook has triggered, recalculate score from baseline
	// to handle external changes (IDE, terminal, git CLI) that reduce the diff
	if sess.StopTriggered {
		// Forgive commits that landed since the baseline (pull/rebase/
		// merge) before judging recovery, so upstream churn can't hold
		// the breaker closed.
		cfg := loadConfig(log)
		maybeRebaseBaseline(sess, cfg.ResetOn, git.HeadCommit(), log)

		currentTree, err := git.CaptureTree()
		if err != nil {
			// Fail-open: If we can't capture tree state, don't block the user
			log.Warn("failed to capture tree for auto-recovery check: %v (failing open)", err)
			return 0
		}

		headTree := git.HeadTree()
		if headTree == "" {
			// Fail-open: If HEAD tree unavailable (empty repo?), don't block
			log.Warn("HEAD tree unavailable for auto-recovery check (failing open)")
			return 0
		}

		if currentTree == headTree {
			// Tree is clean - auto-reset baseline and clear flag
			scoreAtReset := sess.Score
			currentBranch := git.CurrentBranch()
			sess.ResetBaseline(currentTree, currentBranch, git.HeadCommit())
			saveOrLog(sess, log, "auto-reset on clean tree")
			if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.Reset, Score: scoreAtReset, Limit: sess.ThresholdLimit, Cause: events.CauseCleanTree}); err != nil {
				log.Warn("failed to append event: %v", err)
			}

			// Provide feedback to user and Claude
			fmt.Fprintf(os.Stderr, "✓ Baseline auto-reset (external commit detected). Budget restored.\n")
			return 0
		}

		// Tree is dirty - recalculate score to check if below threshold
		// This mirrors the Stop hook's auto-recovery logic (stop.go:152-172)
		stats := getStatsJSON(sess.BaselineTree)
		if stats == nil {
			log.Warn("failed to get diff stats for auto-recovery (failing open)")
			return 0 // Fail open
		}

		result := scoring.Calculate(stats)
		freshScore := result.Score

		if freshScore <= sess.ThresholdLimit {
			// Score at or below threshold - auto-recover
			sess.SetStopTriggered(false)
			sess.SetScore(freshScore)
			sess.NetLines = result.NetLines
			saveOrLog(sess, log, "auto-recovery below threshold")

			pct := 0
			if sess.ThresholdLimit > 0 {
				pct = (freshScore * 100) / sess.ThresholdLimit
			}

			// Provide feedback to user and Claude
			fmt.Fprintf(os.Stderr, "✓ Threshold auto-recovered: %d/%d pts (%d%%). External changes reduced diff.\n",
				freshScore, sess.ThresholdLimit, pct)
			return 0
		}

		// Still over threshold - update score and fall through to blocking
		sess.SetScore(freshScore)
		sess.NetLines = result.NetLines
		saveOrLog(sess, log, "score update while still over threshold")
	}

	// KEY CHECK: Only block if Stop hook has already triggered
	// This ensures we don't prematurely block before the user sees the threshold warning
	if !sess.StopTriggered {
		return 0
	}

	// Stop was triggered and not reset - block the tool
	pct := 0
	if sess.ThresholdLimit > 0 {
		pct = (sess.Score * 100) / sess.ThresholdLimit
	}

	policy := sess.Policy
	if policy == nil {
		cfg := loadConfig(log)
		policy = reviewPolicy(cfg)
	}
	reason := formatBlockReason(sess.Score, sess.ThresholdLimit, pct, policy)

	resp := hookio.PreToolUseResponse{
		HookSpecificOutput: &hookio.HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		},
	}

	if err := hookio.Write(resp); err != nil {
		log.Warn("failed to write response: %v", err)
	}

	return 0 // Exit 0 for JSON output
}

// recordHeadBeforeCommit stores HEAD ahead of a commit-shaped Bash command,
// so PostToolUse can prove the commit landed by seeing HEAD move. Regex on
// the command only decides whether to record; the reset decision itself
// rests on HEAD evidence, never on output scraping.
func recordHeadBeforeCommit(input *hookio.Input, log *logging.Logger) {
	if input.ToolInput == nil || !gitCommitPattern.MatchString(input.ToolInput.Command) {
		return
	}
	if !git.IsRepo() {
		return
	}
	sess, err := state.Load(input.SessionID)
	if err != nil {
		return // No session - nothing to reset later anyway
	}
	sess.HeadBeforeCommit = git.HeadCommit()
	if err := sess.Save(); err != nil {
		log.Warn("failed to record pre-commit HEAD: %v", err)
	}
}

// formatBlockReason creates the denial message shown to Claude. It branches
// on the trip policy: under on_trip: review it scripts the self-review flow
// (the tool-holder here may be a subagent that can act on it directly);
// under block/human policies it names review-with-the-user and committing,
// and leaves /bumper-reset to the human rather than instructing whoever
// holds the tool to run it.
func formatBlockReason(score, limit, pct int, policy *state.ReviewPolicy) string {
	header := `Bumper lanes: File modifications blocked.

Threshold exceeded: ` + formatScore(score, limit, pct) + `

The Stop hook has already fired. `

	if policy != nil && policy.OnTrip == config.OnTripReview {
		return header + "To continue:" + enforce.ReviewNextMove(policy.ReviewCommand) +
			"\nThis prevents unbounded changes without review."
	}

	return header + `To continue:
1. Review changes with the user
2. Commit changes (baseline auto-resets); the user can also run /bumper-reset to restore budget manually

This prevents unbounded changes without review.`
}

// formatScore formats the score display.
func formatScore(score, limit, pct int) string {
	return fmt.Sprintf("%d/%d pts (%d%%)", score, limit, pct)
}

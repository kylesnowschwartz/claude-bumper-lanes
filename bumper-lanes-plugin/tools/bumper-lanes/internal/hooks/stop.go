package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/enforce"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

// stopOutcome is what the shared enforcement core decided at a stop
// boundary. The boundary-specific handlers (Stop, SubagentStop) render it;
// all state transitions have already been saved.
type stopOutcome struct {
	kind       stopOutcomeKind
	sess       *state.SessionState
	result     *scoring.WeightedScore
	stats      *diff.StatsJSON
	freshTrip  bool // this boundary armed the breaker (was not tripped before)
	prevBranch string
	newBranch  string
}

type stopOutcomeKind int

const (
	// stopAllow: nothing to enforce or report (under threshold, paused,
	// disabled, fail-open). Callers return silently.
	stopAllow stopOutcomeKind = iota
	// stopBranchReset: baseline auto-reset because the branch changed.
	stopBranchReset
	// stopRecovered: was tripped, score dropped to or under threshold.
	stopRecovered
	// stopTripped: over threshold; StopTriggered set, policy stamped.
	stopTripped
)

// runStopBoundary is the enforcement core shared by every stop boundary
// (the parent session's Stop and a subagent's SubagentStop): loop guard,
// lock, session load, paused/disabled score tracking, upstream-rebase
// forgiveness, branch-switch reset, fresh score, and the under/over
// threshold state machine. tripCause tags the trip event with which
// boundary detected it ("" = parent Stop).
//
// Returns nil when there is nothing to render (fail-open paths included).
func runStopBoundary(input *hookio.Input, tripCause string, log *logging.Logger) *stopOutcome {
	if !git.IsRepo() {
		return nil
	}

	// Acquire lock to prevent parallel stop boundaries from racing. The
	// session is loaded after the lock so the state snapshot (and every
	// save below) can't clobber a concurrent reset.
	lockDir, err := acquireLock(input.SessionID)
	if err != nil {
		log.Warn("failed to acquire stop lock: %v (failing open, assuming another instance holds it)", err)
		return nil
	}
	defer releaseLock(lockDir)

	// If already blocked once, allow the stop to prevent an infinite loop.
	if input.StopHookActive {
		return nil
	}

	sess, err := state.Load(input.SessionID)
	if err != nil {
		log.Warn("failed to load session state: %v (failing open)", err)
		return nil
	}

	// Always recalculate score to enable bidirectional state transitions.
	// If paused or disabled, track changes but don't enforce.
	if sess.Paused || sess.ThresholdLimit == 0 {
		stats := getStatsJSON(sess.BaselineTree)
		if stats != nil {
			result := scoring.Calculate(stats)
			sess.SetScore(result.Score)
			sess.NetLines = result.NetLines
			saveOrLog(sess, log, "score update while paused or disabled")
		}
		return nil
	}

	// Config is loaded at most once per boundary and only when needed, so
	// the common under-threshold, HEAD-unmoved case never pays for the
	// subprocess + file reads.
	var cfg config.Settings
	var cfgLoaded bool
	getConfig := func() config.Settings {
		if !cfgLoaded {
			cfg = loadConfig(log)
			cfgLoaded = true
		}
		return cfg
	}

	// Forgive commits that landed since the baseline (pull/rebase/merge)
	// before any score calculation below.
	head := git.HeadCommit()
	if head != "none" && head != sess.BaselineHead {
		maybeRebaseBaseline(sess, getConfig().ResetOn, head, log)
	}

	// Detect branch switch - auto-reset baseline.
	currentBranch := git.CurrentBranch()
	if sess.BaselineBranch != "" && currentBranch != "" && sess.BaselineBranch != currentBranch {
		// Only capture tree when actually needed (branch switch detected)
		// This avoids ~50ms overhead on every stop boundary
		currentTree, err := git.CaptureTree()
		if err != nil {
			log.Warn("failed to capture current tree for branch reset: %v (failing open)", err)
			return nil
		}
		prevBranch := sess.BaselineBranch
		scoreAtReset := sess.Score
		sess.ResetBaseline(currentTree, currentBranch, git.HeadCommit())
		saveOrLog(sess, log, "auto-reset on branch switch")
		if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.Reset, Score: scoreAtReset, Limit: sess.ThresholdLimit, Cause: events.CauseBranch}); err != nil {
			log.Warn("failed to append event: %v", err)
		}
		return &stopOutcome{kind: stopBranchReset, sess: sess, prevBranch: prevBranch, newBranch: currentBranch}
	}

	// Get diff stats from baseline (fresh calculation, not incremental)
	// This allows score to decrease when user manually deletes/reverts changes
	stats := getStatsJSON(sess.BaselineTree)
	if stats == nil {
		log.Warn("failed to get diff stats (failing open)")
		return nil
	}

	result := scoring.Calculate(stats)
	freshScore := result.Score

	if freshScore <= sess.ThresholdLimit {
		wasTripped := sess.StopTriggered
		if wasTripped {
			// Automatic recovery: score dropped below threshold
			sess.SetStopTriggered(false)
		}
		sess.SetScore(freshScore)
		sess.NetLines = result.NetLines
		saveOrLog(sess, log, "score update under threshold")
		if wasTripped {
			return &stopOutcome{kind: stopRecovered, sess: sess, result: result, stats: stats}
		}
		return &stopOutcome{kind: stopAllow, sess: sess, result: result, stats: stats}
	}

	// Over threshold - arm the breaker.
	wasTripped := sess.StopTriggered
	sess.SetStopTriggered(true)
	sess.SetScore(freshScore)
	sess.NetLines = result.NetLines
	// Stamp the effective trip policy for review-clear, which runs from
	// the agent's Bash tool and cannot see plugin userConfig env vars.
	sess.Policy = reviewPolicy(getConfig())
	saveOrLog(sess, log, "score update on trip")
	// Log the trip transition only, not every blocked stop while tripped.
	if !wasTripped {
		if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.Trip, Score: freshScore, Limit: sess.ThresholdLimit, Cause: tripCause}); err != nil {
			log.Warn("failed to append event: %v", err)
		}
	}

	return &stopOutcome{kind: stopTripped, sess: sess, result: result, stats: stats, freshTrip: !wasTripped}
}

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
func Stop(input *hookio.Input) error {
	log := logging.New(input.SessionID, "stop")

	out := runStopBoundary(input, "", log)
	if out == nil {
		return nil
	}
	sess := out.sess

	switch out.kind {
	case stopBranchReset:
		resp := hookio.StopResponse{
			Continue:       true,
			SystemMessage:  fmt.Sprintf("↪ Bumper lanes: Branch changed (%s → %s) — baseline auto-reset.", out.prevBranch, out.newBranch),
			SuppressOutput: false,
		}
		return hookio.Write(resp)

	case stopRecovered:
		pct := 0
		if sess.ThresholdLimit > 0 {
			pct = (sess.Score * 100) / sess.ThresholdLimit
		}
		resp := hookio.StopResponse{
			Continue:       true,
			SystemMessage:  fmt.Sprintf("✓ Bumper lanes: Auto-recovered (score dropped to %d/%d - %d%%)", sess.Score, sess.ThresholdLimit, pct),
			SuppressOutput: false,
		}
		return hookio.Write(resp)

	case stopTripped:
		// The trip packet presents the increment at review altitude: modules,
		// decisions, a scripted next move, and the file-level ground truth.
		// The next move depends on the trip policy: self-review when enabled
		// and available this cycle, otherwise the human packet.
		policy := sess.Policy
		nextMove := enforce.HumanNextMove
		if policy.OnTrip == config.OnTripReview {
			switch {
			case policy.MaxAutoReviews >= 0 && sess.AutoReviews >= policy.MaxAutoReviews:
				nextMove = enforce.HumanNextMove + enforce.EscalationNote
			case policy.TripwiresBlockAutoReview && len(sess.Tripwires) > 0:
				nextMove = enforce.HumanNextMove + "\nNote: tripwires fired, so this trip requires the user (tripwires_block_auto_review).\n"
			default:
				nextMove = enforce.ReviewNextMove(policy.ReviewCommand)
			}
		}
		nextMove += staleBaselineNote(sess)
		reason := enforce.BuildTripPacket(sess, out.result, out.stats, nextMove)
		pct := (sess.Score * 100) / sess.ThresholdLimit

		// Desktop notification on the fresh trip only, so an unattended
		// session surfaces the block once instead of on every retried stop.
		notification := ""
		if out.freshTrip {
			notification = enforce.TripNotification(sess.Score, sess.ThresholdLimit)
		}

		// Build response - see function doc comment for explanation of these confusing semantics
		resp := hookio.StopResponse{
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
			ThresholdData: map[string]any{
				"score":                sess.Score,
				"threshold_limit":      sess.ThresholdLimit,
				"threshold_percentage": pct,
				"new_additions":        out.result.NewAdditions,
				"edit_additions":       out.result.EditAdditions,
				"files_touched":        out.result.FilesTouched,
				"scatter_penalty":      out.result.ScatterPenalty,
			},
		}
		return hookio.Write(resp)
	}

	return nil
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

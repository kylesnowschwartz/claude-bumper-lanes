package hooks

import (
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// maybeRebaseBaseline advances the baseline over commits that landed since
// it was captured (pull, rebase, merge, external commits), so already-
// reviewed upstream work is not charged against the review budget. The
// baseline moves by exactly the old-HEAD→new-HEAD delta via a 3-way tree
// merge - never to the working tree - so the session's uncommitted work
// stays counted.
//
// Runs only under reset_on: commit, where a commit is a review event
// anyway. Under stricter policies advancing over the session's own commits
// would bypass them, so those sessions keep the stale baseline and the trip
// packet names it (staleBaselineNote).
//
// A commit the session makes outside the audited Bash path (a script, a
// Makefile target) is forgiven here too, without ResetBaseline's
// bookkeeping: the score drops but the AutoReviews guard stays armed
// (conservative) and the event logs cause=upstream. The audited path
// (recordHeadBeforeCommit/handleBashCommit) runs first and takes precedence.
//
// Fail-open: any git failure or merge conflict leaves the baseline
// unchanged. Legacy state without a usable baseline head ("" from older
// versions, "none" from an unborn branch or a failed rev-parse) adopts the
// current HEAD without rebasing. Saves the state itself on every mutation;
// returns true when the baseline moved.
func maybeRebaseBaseline(sess *state.SessionState, resetOn string, log *logging.Logger) bool {
	head := git.HeadCommit()
	if head == "none" {
		return false
	}
	if sess.BaselineHead == "" || sess.BaselineHead == "none" {
		sess.BaselineHead = head
		saveOrLog(sess, log, "adopt baseline head (legacy state)")
		return false
	}
	if sess.BaselineHead == head {
		return false
	}
	if resetOn != config.ResetOnCommit {
		return false
	}

	oldHeadTree := git.Tree(sess.BaselineHead)
	newHeadTree := git.Tree(head)
	if oldHeadTree == "" || newHeadTree == "" {
		// The old head can vanish (rebase + gc, history rewrite); the
		// baseline can no longer be advanced precisely. Adopt the new
		// head so the staleness doesn't persist forever.
		log.Warn("baseline head %s unresolvable; adopting current HEAD without rebasing", sess.BaselineHead)
		sess.BaselineHead = head
		saveOrLog(sess, log, "adopt baseline head (unresolvable tree)")
		return false
	}
	if oldHeadTree == newHeadTree {
		// HEAD moved without changing content (e.g. an amend of message
		// only): nothing to forgive.
		sess.BaselineHead = head
		saveOrLog(sess, log, "adopt baseline head (no-op HEAD move)")
		return false
	}

	newBaseline, err := git.MergeTrees(oldHeadTree, sess.BaselineTree, newHeadTree)
	if err != nil {
		log.Warn("baseline rebase failed (%v); baseline stays at %s (run /bumper-reset if the incoming changes were already reviewed)", err, shortSHA(sess.BaselineTree))
		return false
	}

	scoreBefore := sess.Score
	oldHead := sess.BaselineHead
	sess.BaselineTree = newBaseline
	sess.BaselineHead = head
	saveOrLog(sess, log, "baseline rebase")
	events.Append(events.Entry{
		SessionID: sess.SessionID,
		Event:     events.Rebase,
		Score:     scoreBefore,
		Limit:     sess.ThresholdLimit,
		Cause:     events.CauseUpstream,
	})
	log.Info("baseline rebased over upstream commits (%s → %s)", shortSHA(oldHead), shortSHA(head))
	return true
}

// staleBaselineNote returns a trip-packet line when HEAD has moved since the
// baseline but the baseline could not (or, under strict reset policies, must
// not) be advanced automatically. The wording stays neutral about who made
// the commits: under reset_on: human they may be the session's own, and the
// policy deliberately withholds the reset. Empty when the baseline is
// current or was never usably recorded.
func staleBaselineNote(sess *state.SessionState) string {
	if sess.BaselineHead == "" || sess.BaselineHead == "none" || sess.BaselineHead == git.HeadCommit() {
		return ""
	}
	return "\nNote: commits landed since the baseline was captured, so this score can\ninclude committed changes. Judge the increment on the files above; the user\ncan run /bumper-reset if the committed portion was already reviewed.\n"
}

// shortSHA abbreviates a SHA for log lines, tolerating short or empty
// values from corrupted state.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

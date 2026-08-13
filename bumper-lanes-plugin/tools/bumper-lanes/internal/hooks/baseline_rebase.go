package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
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
func maybeRebaseBaseline(sess *state.SessionState, log *logging.Logger) bool {
	head := GetHeadCommit()
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
	if config.LoadResetOn() != config.ResetOnCommit {
		return false
	}

	oldHeadTree := revParseTree(sess.BaselineHead)
	newHeadTree := revParseTree(head)
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

	newBaseline, err := mergeTrees(oldHeadTree, sess.BaselineTree, newHeadTree)
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
	if sess.BaselineHead == "" || sess.BaselineHead == "none" || sess.BaselineHead == GetHeadCommit() {
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

// revParseTree resolves a commit-ish to its tree SHA, or "" on failure.
func revParseTree(commitish string) string {
	out, err := exec.Command("git", "rev-parse", commitish+"^{tree}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// mergeTrees 3-way-merges trees in a temp index (base=the tree HEAD had at
// baseline capture, ours=the baseline, theirs=the tree HEAD has now) and
// returns the resulting tree SHA. An unmergeable index (conflicting change
// to the same path) fails; callers keep the old baseline.
func mergeTrees(base, ours, theirs string) (string, error) {
	tmpIndex, err := os.CreateTemp("", "bumper-rebase-index-*")
	if err != nil {
		return "", err
	}
	tmpIndexPath := tmpIndex.Name()
	tmpIndex.Close()
	// read-tree -m refuses a pre-existing zero-byte index file; git must
	// create the index itself at this path.
	os.Remove(tmpIndexPath)
	defer os.Remove(tmpIndexPath)

	gitWithTempIndex := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndexPath)
		return cmd
	}

	if out, err := gitWithTempIndex("read-tree", "-i", "-m", base, ours, theirs).CombinedOutput(); err != nil {
		return "", fmt.Errorf("read-tree: %s", strings.TrimSpace(string(out)))
	}
	out, err := gitWithTempIndex("write-tree").Output()
	if err != nil {
		return "", fmt.Errorf("write-tree: %w", err)
	}
	tree := strings.TrimSpace(string(out))
	if tree == "" {
		return "", fmt.Errorf("empty tree from merge")
	}
	return tree, nil
}

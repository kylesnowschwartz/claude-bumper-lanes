package hooks

import (
	"fmt"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// ReviewClear clears a tripped breaker after an agent self-review
// (on_trip: review). The agent runs this from the Bash tool between
// reviewing the increment and implementing the findings, so the fixes are
// metered as the next increment. With an empty sessionID it uses the most
// recently active session, like `bumper-lanes budget`.
//
// The clear is trust-based by design: the binary cannot prove a review
// happened, so instead every clear is auditable - a reset event with cause
// "review" records the increment size, and the loop guard (one self-review
// per human touchpoint) bounds how far an unreviewed session can run.
func ReviewClear(sessionID string) error {
	var sess *state.SessionState
	var err error
	if sessionID != "" {
		sess, err = state.Load(sessionID)
		if err != nil {
			sess, err = state.LoadLatest()
		}
	} else {
		sess, err = state.LoadLatest()
	}
	if err != nil {
		return fmt.Errorf("no session state: %w", err)
	}

	if config.LoadOnTrip() != config.OnTripReview {
		return fmt.Errorf("review-clear requires on_trip: \"review\" in .bumper-lanes.json (current policy: %s); ask the user to review instead", config.LoadOnTrip())
	}
	if !sess.StopTriggered {
		return fmt.Errorf("breaker is not tripped (score %d/%d); nothing to clear", sess.Score, sess.ThresholdLimit)
	}
	if sess.AutoReviews >= 1 {
		return fmt.Errorf("self-review already used this cycle; this trip requires the user (/bumper-reset or a commit resets the cycle)")
	}
	if config.LoadTripwiresBlockAutoReview() && len(sess.Tripwires) > 0 {
		return fmt.Errorf("tripwires fired this increment (%v) and tripwires_block_auto_review is set; this trip requires the user", sess.Tripwires)
	}

	newTree, err := CaptureTree()
	if err != nil {
		return fmt.Errorf("capturing baseline: %w", err)
	}

	scoreAtClear := sess.Score
	autoReviews := sess.AutoReviews
	sess.ResetBaseline(newTree, GetCurrentBranch())
	sess.AutoReviews = autoReviews + 1
	if err := sess.Save(); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	events.Append(events.Entry{
		SessionID: sess.SessionID,
		Event:     events.Reset,
		Score:     scoreAtClear,
		Limit:     sess.ThresholdLimit,
		Cause:     events.CauseReview,
	})

	fmt.Printf("Breaker cleared after self-review. Fresh budget: %d pts.\nImplement the review findings as the next increment. The next trip requires the user.\n", sess.ThresholdLimit)
	return nil
}

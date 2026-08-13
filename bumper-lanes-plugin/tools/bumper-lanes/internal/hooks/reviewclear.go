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
// "review" records the increment size, and the loop guard (max_auto_reviews
// self-reviews per human touchpoint, default 1) bounds how far an
// unreviewed session can run. max_auto_reviews: -1 removes the cap.
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
		return fmt.Errorf("cannot resolve a session: %w", err)
	}

	if config.LoadOnTrip() != config.OnTripReview {
		return fmt.Errorf("review-clear requires on_trip: \"review\" in .bumper-lanes.json (current policy: %s); ask the user to review instead", config.LoadOnTrip())
	}
	if !sess.StopTriggered {
		return fmt.Errorf("breaker is not tripped (score %d/%d); nothing to clear", sess.Score, sess.ThresholdLimit)
	}
	maxReviews := config.LoadMaxAutoReviews()
	if maxReviews >= 0 && sess.AutoReviews >= maxReviews {
		return fmt.Errorf("self-review limit reached (%d of %d this cycle); this trip requires the user (/bumper-reset or a commit resets the cycle)", sess.AutoReviews, maxReviews)
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
	sess.ResetBaseline(newTree, GetCurrentBranch(), GetHeadCommit())
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

	nextTrip := "The next trip requires the user."
	if maxReviews < 0 {
		nextTrip = "Self-review remains available on the next trip (max_auto_reviews: unlimited)."
	} else if remaining := maxReviews - sess.AutoReviews; remaining > 0 {
		nextTrip = fmt.Sprintf("%d self-review(s) remain this cycle.", remaining)
	}
	fmt.Printf("Breaker cleared after self-review. Fresh budget: %d pts.\nImplement the review findings as the next increment. %s\n", sess.ThresholdLimit, nextTrip)
	return nil
}

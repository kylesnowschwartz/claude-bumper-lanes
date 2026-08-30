package hooks

import (
	"fmt"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/enforce"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
)

// SubagentStop handles the SubagentStop hook event: a subagent finishing is
// a wrap-up boundary, so the shared enforcement core runs here just like at
// the parent's Stop. Without it, a subagent's writes are metered
// (parent-configured PreToolUse/PostToolUse hooks fire for subagent tool
// calls) but the breaker only arms at the parent's Stop, letting a single
// subagent brief overshoot the budget without bound.
//
// The payload's session_id is the parent session's id (the subagent is
// identified by the separate agent_id/agent_type fields), so the normal
// session lookup applies.
//
// Only a fresh trip blocks the subagent's stop, once, with a wrap-up
// instruction: summarize the changes for review, modify nothing further.
// This steers rather than crashes - the subagent lands its thread with a
// review-ready report, and the armed breaker (StopTriggered, enforced by
// the PreToolUse deny in every agent) holds new work for the review. A
// subagent finishing while the session is already tripped - a reviewer or
// explorer that changed nothing - stops undisturbed.
//
// The full trip packet still belongs to the parent's Stop, which fires next.
func SubagentStop(input *hookio.Input) error {
	log := logging.New(input.SessionID, "subagent_stop")

	out := runStopBoundary(input, events.CauseSubagentStop, log)
	if out == nil || out.kind != stopTripped || !out.freshTrip {
		return nil
	}

	sess := out.sess
	pct := (sess.Score * 100) / sess.ThresholdLimit
	resp := hookio.StopResponse{
		// The subagent keeps working just long enough to write its summary.
		Continue: true,
		Decision: "block",
		Reason: fmt.Sprintf(`bumper-lanes: review budget exceeded (%d/%d pts - %d%%). The review breaker is now armed: further file modifications will be denied until the increment is reviewed.

Wrap up now: summarize what you changed and why at review altitude (decisions and risky spots, not a line-by-line recap), report it back, and modify no more files.`, sess.Score, sess.ThresholdLimit, pct),
		// A subagent-caused trip is still a trip the user should hear about
		// once; the parent Stop's notification branch won't fire (the trip
		// is no longer fresh by then).
		TerminalSequence: enforce.TripNotification(sess.Score, sess.ThresholdLimit),
	}
	return hookio.Write(resp)
}

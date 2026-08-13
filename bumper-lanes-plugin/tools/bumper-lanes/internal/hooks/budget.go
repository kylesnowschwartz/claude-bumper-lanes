package hooks

import (
	"fmt"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// remainingBudget returns the unspent review-budget points, clamped at 0
// when the score has run past the limit. Every budget message shares this
// so the three injection sites cannot drift.
func remainingBudget(score, limit int) int {
	remaining := limit - score
	if remaining < 0 {
		return 0
	}
	return remaining
}

// budgetLine formats the shared scope-contract lead-in for budget messages.
func budgetLine(score, limit int) string {
	pct := 0
	if limit > 0 {
		pct = (score * 100) / limit
	}
	return fmt.Sprintf("%d/%d review-budget pts remain (%d%% used)", remainingBudget(score, limit), limit, pct)
}

// Budget prints the plain-text budget for a session to stdout. With an
// empty sessionID it uses the most recently active session, which lets
// Claude check the budget from the Bash tool (no statusline JSON needed).
func Budget(sessionID string) error {
	var sess *state.SessionState
	var err error
	if sessionID != "" {
		sess, err = state.Load(sessionID)
		if err != nil {
			// The session id from the environment may predate the plugin
			// or belong to a session without a checkpoint - fall back.
			sess, err = state.LoadLatest()
		}
	} else {
		sess, err = state.LoadLatest()
	}
	if err != nil {
		return fmt.Errorf("cannot resolve a session: %w", err)
	}

	switch {
	case sess.ThresholdLimit == 0:
		fmt.Println("enforcement disabled (threshold 0)")
	case sess.Paused:
		fmt.Printf("enforcement paused; %s\n", budgetLine(sess.Score, sess.ThresholdLimit))
	default:
		fmt.Println(budgetLine(sess.Score, sess.ThresholdLimit))
	}
	return nil
}

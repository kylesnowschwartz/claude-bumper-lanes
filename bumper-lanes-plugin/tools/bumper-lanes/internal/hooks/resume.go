package hooks

import (
	"fmt"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Resume handles the resume user command.
// It sets paused=false to re-enable enforcement.
func Resume(sessionID string) error {
	sess, err := state.Load(sessionID)
	if err != nil {
		return fmt.Errorf("no session state for %s", sessionID)
	}

	sess.SetPaused(false)

	if err := sess.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	events.Append(events.Entry{SessionID: sessionID, Event: events.Resume, Score: sess.Score, Limit: sess.ThresholdLimit})

	fmt.Println("Enforcement resumed.")
	return nil
}

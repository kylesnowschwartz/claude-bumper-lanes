package hooks

import (
	"fmt"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Pause handles the pause user command.
// It sets paused=true to temporarily disable enforcement.
func Pause(sessionID string) error {
	sess, err := state.Load(sessionID)
	if err != nil {
		return fmt.Errorf("no session state for %s", sessionID)
	}

	sess.SetPaused(true)

	if err := sess.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	events.Append(events.Entry{SessionID: sessionID, Event: events.Pause, Score: sess.Score, Limit: sess.ThresholdLimit})

	fmt.Println("Enforcement paused. Use 'resume' to re-enable.")
	return nil
}

package hooks

import (
	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// SessionEnd handles the SessionEnd hook event.
// It cleans up the session state file.
func SessionEnd(input *HookInput) error {
	// Delete session state - ignore errors (file may not exist)
	state.Delete(input.SessionID)
	return nil
}

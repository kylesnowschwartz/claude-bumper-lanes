package hooks

import (
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// saveOrLog saves session state, logging a WARN naming the caller's context
// on failure. The operation that called it proceeds regardless (fail open).
func saveOrLog(sess *state.SessionState, log *logging.Logger, context string) {
	if err := sess.Save(); err != nil {
		log.Warn("failed to save session state (%s): %v (failing open)", context, err)
	}
}

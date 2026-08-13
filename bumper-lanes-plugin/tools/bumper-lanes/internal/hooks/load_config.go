package hooks

import (
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
)

// loadConfig resolves config.Load and logs every warning it returns (e.g. a
// malformed config file or an unparseable plugin env value) instead of
// discarding them, so operators can find the cause in the session log.
func loadConfig(log *logging.Logger) config.Settings {
	cfg, warnings := config.Load()
	for _, w := range warnings {
		log.Warn("config: %s", w)
	}
	return cfg
}

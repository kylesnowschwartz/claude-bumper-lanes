package hooks

import (
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
)

// loadConfig resolves config.Load and logs every warning it returns (e.g. a
// malformed config file, an unparseable plugin env value, or an invalid
// enum/threshold value replaced by its default) instead of discarding them,
// so operators can find the cause in the session log.
func loadConfig(log *logging.Logger) config.Settings {
	cfg, _ := loadConfigWithWarnings(log)
	return cfg
}

// loadConfigWithWarnings is loadConfig plus the warning text, for the config
// display paths (handleConfig, ConfigShow) that also surface warnings to the
// user, not just the session log.
func loadConfigWithWarnings(log *logging.Logger) (config.Settings, []string) {
	cfg, warnings := config.Load()
	for _, w := range warnings {
		log.Warn("config: %s", w)
	}
	return cfg, warnings
}

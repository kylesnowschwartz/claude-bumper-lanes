// Package config handles configuration loading for bumper-lanes.
// Config files (in precedence order):
//  1. .bumper-lanes.json at repo root (highest priority)
//  2. ~/.config/bumper-lanes/config.json (global fallback)
//  3. Built-in defaults (lowest priority)
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// DefaultThreshold is the default diff point threshold.
const DefaultThreshold = 600

// Reset policies control when a git commit made through the Bash tool
// auto-resets the review budget.
const (
	// ResetOnCommit resets after any commit with success evidence.
	ResetOnCommit = "commit"
	// ResetOnVerifiedCommit additionally refuses commits that bypass
	// hooks with --no-verify / -n.
	ResetOnVerifiedCommit = "verified-commit"
	// ResetOnHuman never auto-resets on Claude's commits; only a human
	// /bumper-reset (or a clean tree) restores the budget.
	ResetOnHuman = "human"

	// DefaultResetOn is the default reset policy.
	DefaultResetOn = ResetOnCommit
)

// Trip policies control what the trip packet asks for when the budget trips.
const (
	// OnTripBlock presents the human review packet (default).
	OnTripBlock = "block"
	// OnTripReview instructs the agent to self-review the increment,
	// clear the breaker with `bumper-lanes review-clear`, then implement
	// the findings as the next increment.
	OnTripReview = "review"

	// DefaultOnTrip is the default trip policy.
	DefaultOnTrip = OnTripBlock

	// DefaultReviewCommand names the review workflow the packet points
	// the agent at when on_trip is "review".
	DefaultReviewCommand = "/code-review"

	// DefaultMaxAutoReviews is the number of self-review clears allowed per
	// human touchpoint: one, so every second trip reaches the user.
	DefaultMaxAutoReviews = 1

	// UnlimitedAutoReviews (max_auto_reviews: -1) removes the per-cycle
	// cap: hands-off mode, where trips force a review but never a human.
	UnlimitedAutoReviews = -1
)

// DefaultTripwirePaths are glob patterns for files whose every change is a
// review-worthy decision regardless of size: CI definitions, dependency
// manifests, agent-harness config, and schema migrations.
var DefaultTripwirePaths = []string{
	".github/workflows/**",
	".gitlab-ci.yml",
	".circleci/**",
	"Jenkinsfile",
	"go.mod",
	"package.json",
	"Gemfile",
	"requirements.txt",
	"pyproject.toml",
	"Cargo.toml",
	".claude/settings*.json",
	"**/hooks.json",
	"db/migrate/**",
	"**/migrations/**",
}

// DefaultTripwirePatterns are substrings that, appearing on an added line,
// signal a silently weakened test suite.
var DefaultTripwirePatterns = []string{
	"t.Skip",
	"it.skip",
	"test.skip",
	"describe.skip",
	"xit(",
	"xdescribe(",
	"@pytest.mark.skip",
	"@unittest.skip",
}

// Config represents bumper-lanes configuration.
// Threshold: nil=use default (600), 0=disabled, 50-2000=active threshold
// StatuslineAutoSetup: nil=default (false), true=allow session-start to configure the status line
// ResetOn: ""=default ("commit"); one of "commit", "verified-commit", "human"
// TripwirePaths/TripwirePatterns: nil=defaults, empty list=disabled
type Config struct {
	Threshold                *int      `json:"threshold,omitempty"`
	StatuslineAutoSetup      *bool     `json:"statusline_auto_setup,omitempty"`
	ResetOn                  string    `json:"reset_on,omitempty"`
	OnTrip                   string    `json:"on_trip,omitempty"`
	ReviewCommand            string    `json:"review_command,omitempty"`
	TripwiresBlockAutoReview *bool     `json:"tripwires_block_auto_review,omitempty"`
	MaxAutoReviews           *int      `json:"max_auto_reviews,omitempty"`
	TripwirePaths            *[]string `json:"tripwire_paths,omitempty"`
	TripwirePatterns         *[]string `json:"tripwire_patterns,omitempty"`
}

// GetGitDir returns the absolute git directory path.
func GetGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--absolute-git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getRepoRoot returns the repository root path.
func getRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// loadConfigFile reads and parses a JSON config file.
func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// getGlobalConfigPath returns the path to the global config file.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config/bumper-lanes/config.json.
func getGlobalConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "bumper-lanes", "config.json")
}

// loadMergedConfig loads config from global and repo locations, merging them.
// Repo config values override global config values.
// Returns an empty Config if neither file exists (never nil).
// loadMergedConfig resolves the effective config. Precedence, lowest first:
// legacy global file (~/.config/bumper-lanes, deprecated) < plugin config
// (userConfig values, via CLAUDE_PLUGIN_OPTION_* env in hook processes)
// < repo .bumper-lanes.json.
func loadMergedConfig() (*Config, []string) {
	var warnings []string
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	merged := &Config{}

	if globalPath := getGlobalConfigPath(); globalPath != "" {
		if global, err := loadConfigFile(globalPath); err == nil {
			merged = global
		} else if !os.IsNotExist(err) {
			warn("%s: %v", globalPath, err)
		}
	}

	overlay(merged, pluginOptionsFromEnv(warn))

	repoRoot, err := getRepoRoot()
	if err != nil {
		return merged, warnings
	}
	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	repo, err := loadConfigFile(repoPath)
	if err != nil {
		if !os.IsNotExist(err) {
			warn("%s: %v", repoPath, err)
		}
		return merged, warnings
	}
	overlay(merged, repo)

	return merged, warnings
}

// overlay applies src's set fields (non-nil pointers, non-empty strings)
// over dst.
func overlay(dst, src *Config) {
	if src.Threshold != nil {
		dst.Threshold = src.Threshold
	}
	if src.StatuslineAutoSetup != nil {
		dst.StatuslineAutoSetup = src.StatuslineAutoSetup
	}
	if src.ResetOn != "" {
		dst.ResetOn = src.ResetOn
	}
	if src.OnTrip != "" {
		dst.OnTrip = src.OnTrip
	}
	if src.ReviewCommand != "" {
		dst.ReviewCommand = src.ReviewCommand
	}
	if src.TripwiresBlockAutoReview != nil {
		dst.TripwiresBlockAutoReview = src.TripwiresBlockAutoReview
	}
	if src.MaxAutoReviews != nil {
		dst.MaxAutoReviews = src.MaxAutoReviews
	}
	if src.TripwirePaths != nil {
		dst.TripwirePaths = src.TripwirePaths
	}
	if src.TripwirePatterns != nil {
		dst.TripwirePatterns = src.TripwirePatterns
	}
}

// pluginOptionsFromEnv builds a Config from the CLAUDE_PLUGIN_OPTION_<KEY>
// variables Claude Code sets in plugin subprocesses for userConfig values
// (plugin.json). Only hook processes see them; CLI invocations from the
// agent's Bash tool read the policy the hooks stamped into session state
// instead. Unparseable values are ignored but reported through warn, so
// they surface in Load's warnings (the enable-time prompt is typed, so
// these should arrive well-formed).
func pluginOptionsFromEnv(warn func(format string, args ...any)) *Config {
	cfg := &Config{}
	if v, ok := envInt("CLAUDE_PLUGIN_OPTION_THRESHOLD", warn); ok {
		cfg.Threshold = &v
	}
	if v, ok := envBool("CLAUDE_PLUGIN_OPTION_STATUSLINE_AUTO_SETUP", warn); ok {
		cfg.StatuslineAutoSetup = &v
	}
	cfg.ResetOn = os.Getenv("CLAUDE_PLUGIN_OPTION_RESET_ON")
	cfg.OnTrip = os.Getenv("CLAUDE_PLUGIN_OPTION_ON_TRIP")
	cfg.ReviewCommand = os.Getenv("CLAUDE_PLUGIN_OPTION_REVIEW_COMMAND")
	if v, ok := envBool("CLAUDE_PLUGIN_OPTION_TRIPWIRES_BLOCK_AUTO_REVIEW", warn); ok {
		cfg.TripwiresBlockAutoReview = &v
	}
	if v, ok := envInt("CLAUDE_PLUGIN_OPTION_MAX_AUTO_REVIEWS", warn); ok {
		cfg.MaxAutoReviews = &v
	}
	return cfg
}

// HasPluginOptions reports whether any plugin userConfig value is present
// in the environment. True only in plugin subprocesses (hook processes)
// where the user has set values via the plugin's enable-time prompts.
func HasPluginOptions() bool {
	for _, key := range []string{"THRESHOLD", "RESET_ON", "ON_TRIP", "MAX_AUTO_REVIEWS", "REVIEW_COMMAND", "TRIPWIRES_BLOCK_AUTO_REVIEW", "STATUSLINE_AUTO_SETUP"} {
		if os.Getenv("CLAUDE_PLUGIN_OPTION_"+key) != "" {
			return true
		}
	}
	return false
}

func envInt(key string, warn func(format string, args ...any)) (int, bool) {
	raw := os.Getenv(key)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		warn("%s: invalid integer %q (ignoring)", key, raw)
		return 0, false
	}
	return v, true
}

func envBool(key string, warn func(format string, args ...any)) (bool, bool) {
	raw := os.Getenv(key)
	switch raw {
	case "":
		return false, false
	case "true":
		return true, true
	case "false":
		return false, true
	}
	warn("%s: invalid boolean %q, want \"true\" or \"false\" (ignoring)", key, raw)
	return false, false
}

// Settings is the resolved configuration: every field carries an effective
// value, with defaults applied and invalid values replaced. The pointer-based
// file shape (Config) stays internal to loading and merging.
type Settings struct {
	Threshold                int
	StatuslineAutoSetup      bool
	ResetOn                  string
	OnTrip                   string
	ReviewCommand            string
	MaxAutoReviews           int
	TripwiresBlockAutoReview bool
	TripwirePaths            []string
	TripwirePatterns         []string
}

// Load reads and merges the config sources (legacy global file < plugin
// userConfig env < repo .bumper-lanes.json) once and resolves every value.
// The returned warnings name malformed config files (parse/read errors
// other than "file does not exist") and unparseable plugin env values;
// config must not import internal/logging (it stays a leaf package), so
// hook callers log them. Resolution:
//   - Threshold: DefaultThreshold when unset; 0 = disabled.
//   - StatuslineAutoSetup: opt-in, defaults to false, because it rewrites
//     user-global settings from whatever repo runs the hook.
//   - ResetOn: unknown values fall back to the default rather than silently
//     disabling resets.
//   - OnTrip: unknown values fall back to the default (block) rather than
//     silently enabling self-clearing.
//   - MaxAutoReviews: N per cycle, 0 = never (same as on_trip: block), any
//     negative value = UnlimitedAutoReviews (hands-off mode).
//   - TripwirePaths/TripwirePatterns: nil config = defaults; an explicit
//     empty list disables that tripwire lane.
func Load() (Settings, []string) {
	cfg, warnings := loadMergedConfig()
	s := Settings{
		Threshold:        DefaultThreshold,
		ResetOn:          DefaultResetOn,
		OnTrip:           DefaultOnTrip,
		ReviewCommand:    DefaultReviewCommand,
		MaxAutoReviews:   DefaultMaxAutoReviews,
		TripwirePaths:    DefaultTripwirePaths,
		TripwirePatterns: DefaultTripwirePatterns,
	}
	if cfg.Threshold != nil {
		s.Threshold = *cfg.Threshold
	}
	if cfg.StatuslineAutoSetup != nil {
		s.StatuslineAutoSetup = *cfg.StatuslineAutoSetup
	}
	switch cfg.ResetOn {
	case ResetOnCommit, ResetOnVerifiedCommit, ResetOnHuman:
		s.ResetOn = cfg.ResetOn
	}
	switch cfg.OnTrip {
	case OnTripBlock, OnTripReview:
		s.OnTrip = cfg.OnTrip
	}
	if cfg.ReviewCommand != "" {
		s.ReviewCommand = cfg.ReviewCommand
	}
	if cfg.MaxAutoReviews != nil {
		s.MaxAutoReviews = *cfg.MaxAutoReviews
		if s.MaxAutoReviews < 0 {
			s.MaxAutoReviews = UnlimitedAutoReviews
		}
	}
	if cfg.TripwiresBlockAutoReview != nil {
		s.TripwiresBlockAutoReview = *cfg.TripwiresBlockAutoReview
	}
	if cfg.TripwirePaths != nil {
		s.TripwirePaths = *cfg.TripwirePaths
	}
	if cfg.TripwirePatterns != nil {
		s.TripwirePatterns = *cfg.TripwirePatterns
	}
	return s, warnings
}

// IsDisabled returns true if the given threshold means enforcement is disabled.
func IsDisabled(threshold int) bool {
	return threshold == 0
}

// knownConfigKeys are the keys the current config schema understands.
var knownConfigKeys = map[string]bool{
	"threshold":                   true,
	"statusline_auto_setup":       true,
	"reset_on":                    true,
	"on_trip":                     true,
	"review_command":              true,
	"tripwires_block_auto_review": true,
	"max_auto_reviews":            true,
	"tripwire_paths":              true,
	"tripwire_patterns":           true,
}

// UnknownKeys returns the keys in a config file that the current schema does
// not understand (e.g. options removed in a previous major version). Config
// rot is otherwise silent: unknown keys are ignored by json.Unmarshal.
// Returns nil when the file is missing or unreadable.
func UnknownKeys(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var unknown []string
	for key := range raw {
		if !knownConfigKeys[key] {
			unknown = append(unknown, key)
		}
	}
	slices.Sort(unknown)
	return unknown
}

// GetConfigPath returns the path to .bumper-lanes.json (or empty if not in a repo).
func GetConfigPath() string {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return ""
	}
	return filepath.Join(repoRoot, ".bumper-lanes.json")
}

// GetGlobalConfigPath returns the path to the global config file.
// Exported for documentation and debugging.
func GetGlobalConfigPath() string {
	return getGlobalConfigPath()
}

// SaveRepoConfig writes threshold to repo config file.
func SaveRepoConfig(threshold int) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	cfg := Config{Threshold: &threshold}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(repoRoot, ".bumper-lanes.json")
	return os.WriteFile(path, data, 0644)
}

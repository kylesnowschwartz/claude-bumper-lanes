// Package config handles configuration loading for bumper-lanes.
// Config files (in precedence order):
//  1. .bumper-lanes.json at repo root (highest priority)
//  2. ~/.config/bumper-lanes/config.json (global fallback)
//  3. Built-in defaults (lowest priority)
package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
func loadMergedConfig() *Config {
	merged := &Config{}

	// Load global config first (lower priority)
	if globalPath := getGlobalConfigPath(); globalPath != "" {
		if global, err := loadConfigFile(globalPath); err == nil {
			merged = global
		}
	}

	// Load repo config and override (higher priority)
	repoRoot, err := getRepoRoot()
	if err != nil {
		return merged
	}
	repoPath := filepath.Join(repoRoot, ".bumper-lanes.json")
	repo, err := loadConfigFile(repoPath)
	if err != nil {
		return merged
	}

	// Merge: repo values override global (non-nil pointers and non-empty strings)
	if repo.Threshold != nil {
		merged.Threshold = repo.Threshold
	}
	if repo.StatuslineAutoSetup != nil {
		merged.StatuslineAutoSetup = repo.StatuslineAutoSetup
	}
	if repo.ResetOn != "" {
		merged.ResetOn = repo.ResetOn
	}
	if repo.OnTrip != "" {
		merged.OnTrip = repo.OnTrip
	}
	if repo.ReviewCommand != "" {
		merged.ReviewCommand = repo.ReviewCommand
	}
	if repo.TripwiresBlockAutoReview != nil {
		merged.TripwiresBlockAutoReview = repo.TripwiresBlockAutoReview
	}
	if repo.MaxAutoReviews != nil {
		merged.MaxAutoReviews = repo.MaxAutoReviews
	}
	if repo.TripwirePaths != nil {
		merged.TripwirePaths = repo.TripwirePaths
	}
	if repo.TripwirePatterns != nil {
		merged.TripwirePatterns = repo.TripwirePatterns
	}

	return merged
}

// LoadThreshold returns the configured threshold value.
// Checks repo config first, then global config, then returns DefaultThreshold.
// Returns 0 if explicitly disabled.
func LoadThreshold() int {
	cfg := loadMergedConfig()
	if cfg.Threshold != nil {
		return *cfg.Threshold
	}
	return DefaultThreshold
}

// IsDisabled returns true if the given threshold means enforcement is disabled.
func IsDisabled(threshold int) bool {
	return threshold == 0
}

// LoadStatuslineAutoSetup returns whether session-start may configure the
// user's status line in ~/.claude/settings.json. Opt-in: defaults to false,
// because it rewrites user-global settings from whatever repo runs the hook.
func LoadStatuslineAutoSetup() bool {
	cfg := loadMergedConfig()
	if cfg.StatuslineAutoSetup != nil {
		return *cfg.StatuslineAutoSetup
	}
	return false
}

// LoadResetOn returns the configured commit auto-reset policy.
// Unknown values fall back to the default rather than silently
// disabling resets.
func LoadResetOn() string {
	cfg := loadMergedConfig()
	switch cfg.ResetOn {
	case ResetOnCommit, ResetOnVerifiedCommit, ResetOnHuman:
		return cfg.ResetOn
	}
	return DefaultResetOn
}

// LoadTripwirePaths returns the configured tripwire path globs.
// nil config = defaults; an explicit empty list disables path tripwires.
func LoadTripwirePaths() []string {
	cfg := loadMergedConfig()
	if cfg.TripwirePaths != nil {
		return *cfg.TripwirePaths
	}
	return DefaultTripwirePaths
}

// LoadTripwirePatterns returns the configured added-line tripwire patterns.
// nil config = defaults; an explicit empty list disables pattern tripwires.
func LoadTripwirePatterns() []string {
	cfg := loadMergedConfig()
	if cfg.TripwirePatterns != nil {
		return *cfg.TripwirePatterns
	}
	return DefaultTripwirePatterns
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
	sort.Strings(unknown)
	return unknown
}

// LoadOnTrip returns the configured trip policy. Unknown values fall back
// to the default (block) rather than silently enabling self-clearing.
func LoadOnTrip() string {
	cfg := loadMergedConfig()
	switch cfg.OnTrip {
	case OnTripBlock, OnTripReview:
		return cfg.OnTrip
	}
	return DefaultOnTrip
}

// LoadReviewCommand returns the review workflow named in the self-review
// trip packet.
func LoadReviewCommand() string {
	cfg := loadMergedConfig()
	if cfg.ReviewCommand != "" {
		return cfg.ReviewCommand
	}
	return DefaultReviewCommand
}

// LoadMaxAutoReviews returns the self-review clears allowed per human
// touchpoint: N per cycle, 0 = never (same as on_trip: block), any negative
// value = unlimited (hands-off mode).
func LoadMaxAutoReviews() int {
	cfg := loadMergedConfig()
	if cfg.MaxAutoReviews == nil {
		return DefaultMaxAutoReviews
	}
	if *cfg.MaxAutoReviews < 0 {
		return UnlimitedAutoReviews
	}
	return *cfg.MaxAutoReviews
}

// LoadTripwiresBlockAutoReview reports whether tripwire hits exclude an
// increment from self-review clearing (forcing the human packet).
// Default false: the self-review covers tripwires, named as priority items.
func LoadTripwiresBlockAutoReview() bool {
	cfg := loadMergedConfig()
	if cfg.TripwiresBlockAutoReview != nil {
		return *cfg.TripwiresBlockAutoReview
	}
	return false
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

// Package hooks provides prompt handling for bumper-lanes slash commands.
// Commands are intercepted via UserPromptSubmit hook and handled directly
// without invoking the Claude API - output is shown via "block" decision.
package hooks

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Command patterns - regex only for commands that need capture groups.
// Simple commands use matchCommand() with string matching for performance.
var configCmdPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-config\s*(.*)$`)

// matchCommand checks if prompt matches a bumper-lanes command.
// Handles both /bumper-X and /claude-bumper-lanes:bumper-X forms.
// Returns true if the command matches (exact match, no trailing args).
func matchCommand(prompt, cmdName string) bool {
	shortForm := "/" + cmdName
	longForm := "/claude-bumper-lanes:" + cmdName
	return prompt == shortForm || prompt == longForm
}

// HandlePrompt handles slash commands before Claude API execution.
// Returns exit code 0 in all cases (success or handled error).
// Uses JSON output to stdout with decision="block" to show output.
func HandlePrompt(input *hookio.Input) int {
	prompt := strings.TrimSpace(input.GetPrompt())
	if prompt == "" {
		return 0
	}

	// Early exit if not in a git repository - bumper-lanes commands require git
	if !git.IsRepo() {
		return 0 // Pass through - not in a git repo
	}

	sessionID := input.SessionID

	// Simple commands (no args) - use string matching for performance
	if matchCommand(prompt, "bumper-reset") {
		return handleReset(sessionID)
	}
	if matchCommand(prompt, "bumper-pause") {
		return handlePause(sessionID)
	}
	if matchCommand(prompt, "bumper-resume") {
		return handleResume(sessionID)
	}

	if matchCommand(prompt, "bumper-diff") {
		return handleDiff(sessionID)
	}

	// Commands with capture groups - use regex
	if m := configCmdPattern.FindStringSubmatch(prompt); m != nil {
		return handleConfig(sessionID, strings.TrimSpace(m[1]))
	}

	// No match - let it through, injecting budget context when consumption is high
	injectBudgetContext(sessionID)
	return 0
}

// injectBudgetContext adds a budget line to Claude's context at prompt time
// when at least half the review budget is spent, so the model plans the next
// increment to fit. Uses the cached score (no git calls on the prompt path).
func injectBudgetContext(sessionID string) {
	if sessionID == "" {
		return
	}
	log := logging.New(sessionID, "prompt_handler")
	sess, err := state.Load(sessionID)
	if err != nil {
		log.Warn("failed to load session for budget context: %v (failing open)", err)
		return
	}
	if sess.Paused || sess.ThresholdLimit == 0 {
		return
	}
	pct := (sess.Score * 100) / sess.ThresholdLimit
	if pct < 50 {
		return
	}
	if err := hookio.WriteContext("UserPromptSubmit", fmt.Sprintf(
		"bumper-lanes: %s. Plan work that fits the remaining budget, or ask before expanding scope.",
		budgetLine(sess.Score, sess.ThresholdLimit))); err != nil {
		log.Warn("failed to write budget context: %v (failing open)", err)
	}
}

// handleReset captures new baseline and resets score.
func handleReset(sessionID string) int {
	log := logging.New(sessionID, "prompt_handler")
	sess := loadSessionOrBlock(sessionID)
	if sess == nil {
		return 0
	}

	if err := events.Append(events.Entry{SessionID: sessionID, Event: events.Reset, Score: sess.Score, Limit: sess.ThresholdLimit, Cause: events.CauseManual}); err != nil {
		log.Warn("failed to append event: %v", err)
	}

	// Reset score FIRST for immediate statusline update
	sess.Score = 0
	sess.NetLines = 0
	sess.StopTriggered = false
	sess.Tripwires = nil
	sess.AutoReviews = 0
	if !saveOrBlock(sess) {
		return 0
	}

	// Now do the slow git work (statusline already shows 0)
	newTree, err := git.CaptureTree()
	if err != nil {
		// Score already reset - just warn about baseline
		blockPrompt(fmt.Sprintf("Score reset. Warning: baseline capture failed: %v", err))
		return 0
	}

	// Update baseline with new tree, anchored at today's HEAD so later
	// pulls/rebases can be forgiven (maybeRebaseBaseline)
	sess.BaselineTree = newTree
	sess.BaselineHead = git.HeadCommit()
	if branch := git.CurrentBranch(); branch != "" {
		sess.BaselineBranch = branch
	}
	saveOrLog(sess, log, "baseline save after reset")

	blockPrompt(fmt.Sprintf("Baseline reset. Score: 0/%d", sess.ThresholdLimit))
	return 0
}

// handlePause disables threshold enforcement.
func handlePause(sessionID string) int {
	sess := loadSessionOrBlock(sessionID)
	if sess == nil {
		return 0
	}

	sess.SetPaused(true)
	if !saveOrBlock(sess) {
		return 0
	}
	if err := events.Append(events.Entry{SessionID: sessionID, Event: events.Pause, Score: sess.Score, Limit: sess.ThresholdLimit}); err != nil {
		logging.New(sessionID, "prompt_handler").Warn("failed to append event: %v", err)
	}

	blockPrompt("Enforcement paused. Changes still tracked.\nUse /bumper-resume to re-enable.")
	return 0
}

// handleResume re-enables threshold enforcement.
func handleResume(sessionID string) int {
	sess := loadSessionOrBlock(sessionID)
	if sess == nil {
		return 0
	}

	sess.SetPaused(false)
	if !saveOrBlock(sess) {
		return 0
	}
	if err := events.Append(events.Entry{SessionID: sessionID, Event: events.Resume, Score: sess.Score, Limit: sess.ThresholdLimit}); err != nil {
		logging.New(sessionID, "prompt_handler").Warn("failed to append event: %v", err)
	}

	blockPrompt(fmt.Sprintf("Enforcement resumed. Score: %d/%d", sess.Score, sess.ThresholdLimit))
	return 0
}

// handleDiff prints the diff visualization (working tree vs review
// baseline) directly in the transcript. On-demand rendering replaces the
// old always-on statusline visualization and its per-mode command family.
func handleDiff(sessionID string) int {
	sess := loadSessionOrBlock(sessionID)
	if sess == nil {
		return 0
	}

	rendered, err := renderBaselineDiff(sess.BaselineTree)
	if err != nil {
		blockPrompt(fmt.Sprintf("Error rendering diff: %v", err))
		return 0
	}
	if rendered == "" {
		blockPrompt("No changes against the review baseline.")
		return 0
	}
	blockPrompt(rendered)
	return 0
}

// handleConfig shows or sets threshold configuration.
func handleConfig(sessionID, args string) int {
	if args == "" {
		// Show current config
		cfg, cfgWarnings := loadConfigWithWarnings(logging.New(sessionID, "prompt_handler"))
		threshold := cfg.Threshold

		// Attribute the source by which config sources are present,
		// matching ConfigShow - an explicit value equal to the default
		// is still attributed to its source.
		source := "default"
		if config.HasPluginOptions() {
			source = "plugin config (/plugin > claude-bumper-lanes)"
		}
		if repoPath := config.GetConfigPath(); repoPath != "" && fileExists(repoPath) {
			source = repoPath
		}

		var thresholdStr string
		if threshold == 0 {
			thresholdStr = "disabled"
		} else {
			thresholdStr = fmt.Sprintf("%d points", threshold)
		}

		msg := fmt.Sprintf("Threshold: %s\nReset policy: %s\nSource: %s", thresholdStr, cfg.ResetOn, source)
		if w := config.LegacyGlobalConfigWarning(); w != "" {
			msg += fmt.Sprintf("\nWarning: %s", w)
		}
		for _, w := range cfgWarnings {
			msg += fmt.Sprintf("\nWarning: %s", w)
		}
		msg += unknownKeyWarnings()
		blockPrompt(msg)
		return 0
	}

	// Direct number sets config
	return setThreshold(sessionID, args)
}

// unknownKeyWarnings names config keys the current schema does not
// understand in the repo config file. Unknown keys are otherwise ignored
// silently, so options removed across versions rot in place invisibly.
func unknownKeyWarnings() string {
	path := config.GetConfigPath()
	if path == "" {
		return ""
	}
	if unknown := config.UnknownKeys(path); len(unknown) > 0 {
		return fmt.Sprintf("\nWarning: %s has unrecognized keys: %s", path, strings.Join(unknown, ", "))
	}
	return ""
}

// setThreshold parses and saves threshold value to .bumper-lanes.json.
// Accepts 0 (disabled) or 50-2000 (active threshold).
func setThreshold(sessionID, valStr string) int {
	val, err := strconv.Atoi(strings.TrimSpace(valStr))
	if err != nil {
		blockPrompt(fmt.Sprintf("Invalid threshold: %s\nUse 0 (disabled) or 50-2000", valStr))
		return 0
	}

	// Allow 0 (disabled) or 50-2000 (active)
	if val != 0 && (val < 50 || val > 2000) {
		blockPrompt(fmt.Sprintf("Threshold must be 0 (disabled) or 50-2000 (got %d)", val))
		return 0
	}

	if err := config.SaveRepoConfig(val); err != nil {
		blockPrompt(fmt.Sprintf("Error: Failed to save config: %v", err))
		return 0
	}

	// Apply to current session immediately
	if sess := loadSessionOrBlock(sessionID); sess != nil {
		sess.ThresholdLimit = val
		saveOrLog(sess, logging.New(sessionID, "prompt_handler"), "apply threshold to session")
	}

	if val == 0 {
		blockPrompt("Threshold disabled. Run /bumper-config <num> to re-enable.")
	} else {
		blockPrompt(fmt.Sprintf("Threshold set to %d.", val))
	}
	return 0
}

// blockPrompt outputs a JSON response that blocks the prompt and shows reason to user.
func blockPrompt(reason string) {
	resp := hookio.UserPromptResponse{
		Decision: "block",
		Reason:   reason,
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}

// loadSessionOrBlock loads session state, blocking with error message on failure.
// Returns nil if session couldn't be loaded (error already shown to user).
func loadSessionOrBlock(sessionID string) *state.SessionState {
	if sessionID == "" {
		blockPrompt("Error: No session ID available")
		return nil
	}
	sess, err := state.Load(sessionID)
	if err != nil {
		logging.New(sessionID, "prompt_handler").Warn("failed to load session state: %v", err)
		blockPrompt(fmt.Sprintf("Error: No session state for %s", sessionID))
		return nil
	}
	return sess
}

// saveOrBlock saves session state, blocking with error message on failure.
// Returns false if save failed (error already shown to user).
func saveOrBlock(sess *state.SessionState) bool {
	if err := sess.Save(); err != nil {
		blockPrompt(fmt.Sprintf("Error: Failed to save state: %v", err))
		return false
	}
	return true
}

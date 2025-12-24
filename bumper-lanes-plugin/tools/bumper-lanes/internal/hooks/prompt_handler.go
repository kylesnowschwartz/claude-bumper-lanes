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
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Command patterns - match both /bumper-X and /claude-bumper-lanes:bumper-X
var (
	resetCmdPattern  = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-reset\s*$`)
	pauseCmdPattern  = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-pause\s*$`)
	resumeCmdPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-resume\s*$`)
	viewCmdPattern   = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-view\s*(.*)$`)
	configCmdPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-config\s*(.*)$`)
)

// UserPromptResponse is the JSON structure for UserPromptSubmit hook output.
// decision="block" + reason="message" shows output to user without API call.
type UserPromptResponse struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// HandlePrompt handles slash commands before Claude API execution.
// Returns exit code 0 in all cases (success or handled error).
// Uses JSON output to stdout with decision="block" to show output.
func HandlePrompt(input *HookInput) int {
	prompt := strings.TrimSpace(input.GetPrompt())
	if prompt == "" {
		return 0
	}

	sessionID := input.SessionID

	// Try each command pattern
	if resetCmdPattern.MatchString(prompt) {
		return handleReset(sessionID)
	}
	if pauseCmdPattern.MatchString(prompt) {
		return handlePause(sessionID)
	}
	if resumeCmdPattern.MatchString(prompt) {
		return handleResume(sessionID)
	}
	if m := viewCmdPattern.FindStringSubmatch(prompt); m != nil {
		return handleView(sessionID, strings.TrimSpace(m[1]))
	}
	if m := configCmdPattern.FindStringSubmatch(prompt); m != nil {
		return handleConfig(strings.TrimSpace(m[1]))
	}

	// No match - let it through
	return 0
}

// handleReset captures new baseline and resets score.
func handleReset(sessionID string) int {
	sess := loadSessionOrBlock(sessionID)
	if sess == nil {
		return 0
	}

	newTree, err := CaptureTree()
	if err != nil {
		blockPrompt(fmt.Sprintf("Error: Failed to capture tree: %v", err))
		return 0
	}

	sess.ResetBaseline(newTree, GetCurrentBranch())
	if !saveOrBlock(sess) {
		return 0
	}

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

	blockPrompt(fmt.Sprintf("Enforcement resumed. Score: %d/%d", sess.Score, sess.ThresholdLimit))
	return 0
}

// handleView sets or shows the visualization mode.
func handleView(sessionID, mode string) int {
	if mode == "" {
		// Show current mode + hint
		currentMode := config.LoadViewMode()
		blockPrompt(fmt.Sprintf("Current: %s\nModes: tree, collapsed, smart, topn, icicle, brackets", currentMode))
		return 0
	}

	// Validate mode before loading session
	validModes := strings.Fields(config.ValidModes)
	isValid := false
	for _, v := range validModes {
		if mode == v {
			isValid = true
			break
		}
	}
	if !isValid {
		blockPrompt(fmt.Sprintf("Invalid mode: %s\nValid modes: %s", mode, config.ValidModes))
		return 0
	}

	sess := loadSessionOrBlock(sessionID)
	if sess == nil {
		return 0
	}

	sess.SetViewMode(mode)
	if !saveOrBlock(sess) {
		return 0
	}

	// Persist to config (uses existing function from view.go)
	_ = persistViewModeToConfig(mode)

	blockPrompt(fmt.Sprintf("View mode set to: %s", mode))
	return 0
}

// handleConfig shows or sets threshold configuration.
func handleConfig(args string) int {
	if args == "" {
		// Show current config
		threshold := config.LoadThreshold()
		viewMode := config.LoadViewMode()
		source := "default"
		if threshold != config.DefaultThreshold {
			source = "config file"
		}
		blockPrompt(fmt.Sprintf("Threshold: %d points\nView mode: %s\nSource: %s", threshold, viewMode, source))
		return 0
	}

	// Check for "personal <value>" syntax
	if strings.HasPrefix(args, "personal ") {
		valStr := strings.TrimPrefix(args, "personal ")
		return setThreshold(valStr, true)
	}

	// Direct number sets repo config
	return setThreshold(args, false)
}

// setThreshold parses and saves threshold value.
// personal=true saves to .git/bumper-config.json, false saves to .bumper-lanes.json
func setThreshold(valStr string, personal bool) int {
	val, err := strconv.Atoi(strings.TrimSpace(valStr))
	if err != nil {
		blockPrompt(fmt.Sprintf("Invalid threshold: %s\nUse a number 50-2000", valStr))
		return 0
	}

	if val < 50 || val > 2000 {
		blockPrompt(fmt.Sprintf("Threshold must be 50-2000 (got %d)", val))
		return 0
	}

	var saveErr error
	var location string
	if personal {
		saveErr = config.SavePersonalConfig(val)
		location = "personal (.git/bumper-config.json)"
	} else {
		saveErr = config.SaveRepoConfig(val)
		location = "repo (.bumper-lanes.json)"
	}

	if saveErr != nil {
		blockPrompt(fmt.Sprintf("Error: Failed to save config: %v", saveErr))
		return 0
	}

	blockPrompt(fmt.Sprintf("Threshold set to %d (%s).\nRun /bumper-reset to apply to current session.", val, location))
	return 0
}

// blockPrompt outputs a JSON response that blocks the prompt and shows reason to user.
func blockPrompt(reason string) {
	resp := UserPromptResponse{
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

// Package hooks provides prompt handling for bumper-lanes slash commands.
// Commands are intercepted via UserPromptSubmit hook and handled directly
// without invoking the Claude API - output is shown via "block" decision.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Command patterns - regex only for commands that need capture groups.
// Simple commands use matchCommand() with string matching for performance.
var (
	viewCmdPattern   = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-view\s*(.*)$`)
	configCmdPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-config\s*(.*)$`)
)

// matchCommand checks if prompt matches a bumper-lanes command.
// Handles both /bumper-X and /claude-bumper-lanes:bumper-X forms.
// Returns true if the command matches (exact match, no trailing args).
func matchCommand(prompt, cmdName string) bool {
	shortForm := "/" + cmdName
	longForm := "/claude-bumper-lanes:" + cmdName
	return prompt == shortForm || prompt == longForm
}

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

	// Commands with capture groups - use regex
	if m := viewCmdPattern.FindStringSubmatch(prompt); m != nil {
		return handleView(sessionID, strings.TrimSpace(m[1]))
	}
	if m := configCmdPattern.FindStringSubmatch(prompt); m != nil {
		return handleConfig(sessionID, strings.TrimSpace(m[1]))
	}

	// Per-mode commands (no-arg = immediate statusline refresh in Claude Code)
	// Matches diff-viz v2.0.0 modes: tree, smart, sparkline-tree, hotpath, icicle, brackets, gauge, depth, heatmap, stat
	if matchCommand(prompt, "bumper-tree") {
		return handleViewMode(sessionID, "tree")
	}
	if matchCommand(prompt, "bumper-smart") {
		return handleViewMode(sessionID, "smart")
	}
	if matchCommand(prompt, "bumper-sparkline-tree") {
		return handleViewMode(sessionID, "sparkline-tree")
	}
	if matchCommand(prompt, "bumper-hotpath") {
		return handleViewMode(sessionID, "hotpath")
	}
	if matchCommand(prompt, "bumper-icicle") {
		return handleViewMode(sessionID, "icicle")
	}
	if matchCommand(prompt, "bumper-brackets") {
		return handleViewMode(sessionID, "brackets")
	}
	if matchCommand(prompt, "bumper-gauge") {
		return handleViewMode(sessionID, "gauge")
	}
	if matchCommand(prompt, "bumper-depth") {
		return handleViewMode(sessionID, "depth")
	}
	if matchCommand(prompt, "bumper-heatmap") {
		return handleViewMode(sessionID, "heatmap")
	}
	if matchCommand(prompt, "bumper-stat") {
		return handleViewMode(sessionID, "stat")
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
// Note: /bumper-view <mode> won't trigger immediate statusline refresh due to Claude Code bug.
// Use per-mode commands (/bumper-tree, /bumper-icicle, etc.) for instant updates.
func handleView(sessionID, mode string) int {
	if mode == "" {
		// Show current mode + hint
		currentMode := config.LoadViewMode()
		blockPrompt(fmt.Sprintf("Current: %s\nModes: %s", currentMode, config.ValidModes))
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

	// Persist to config for future sessions
	_ = config.SaveConfig(config.Config{DefaultViewMode: mode})

	blockPrompt(fmt.Sprintf("View mode set to: %s", mode))
	return 0
}

// handleViewMode sets view mode via no-arg command (triggers immediate statusline refresh).
// This exists because Claude Code only refreshes statusline for no-arg commands.
func handleViewMode(sessionID, mode string) int {
	sess := loadSessionOrBlock(sessionID)
	if sess == nil {
		return 0
	}

	sess.SetViewMode(mode)
	if !saveOrBlock(sess) {
		return 0
	}

	_ = config.SaveConfig(config.Config{DefaultViewMode: mode})
	blockPrompt(fmt.Sprintf("View: %s", mode))
	return 0
}

// handleConfig shows or sets threshold configuration.
func handleConfig(sessionID, args string) int {
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

	// Direct number sets config
	return setThreshold(sessionID, args)
}

// setThreshold parses and saves threshold value to .bumper-lanes.json.
func setThreshold(sessionID, valStr string) int {
	val, err := strconv.Atoi(strings.TrimSpace(valStr))
	if err != nil {
		blockPrompt(fmt.Sprintf("Invalid threshold: %s\nUse a number 50-2000", valStr))
		return 0
	}

	if val < 50 || val > 2000 {
		blockPrompt(fmt.Sprintf("Threshold must be 50-2000 (got %d)", val))
		return 0
	}

	if err := config.SaveRepoConfig(val); err != nil {
		blockPrompt(fmt.Sprintf("Error: Failed to save config: %v", err))
		return 0
	}

	// Apply to current session immediately
	if sess := loadSessionOrBlock(sessionID); sess != nil {
		sess.ThresholdLimit = val
		sess.Save()
	}

	blockPrompt(fmt.Sprintf("Threshold set to %d.", val))
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

// getBumperLanesBinPath returns the path to the bumper-lanes binary.
func getBumperLanesBinPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "bumper-lanes" // fallback to PATH
	}
	return exe
}

// hasStatusLineConfigured is also used by session_start.go
// (imported from session_start.go)

// Package hooks provides prompt handling for bumper-lanes slash commands.
// Commands are intercepted via UserPromptSubmit hook and handled directly
// without invoking the Claude API - output is shown via "block" decision.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// Command patterns - match both /bumper-X and /claude-bumper-lanes:bumper-X
var (
	resetCmdPattern           = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-reset\s*$`)
	pauseCmdPattern           = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-pause\s*$`)
	resumeCmdPattern          = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-resume\s*$`)
	viewCmdPattern            = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-view\s*(.*)$`)
	configCmdPattern          = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-config\s*(.*)$`)
	setupStatuslineCmdPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-setup-statusline\s*$`)
	// Per-mode commands (no-arg = immediate statusline refresh in Claude Code)
	viewTreePattern      = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-tree\s*$`)
	viewIciclePattern    = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-icicle\s*$`)
	viewCollapsedPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-collapsed\s*$`)
	viewSmartPattern     = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-smart\s*$`)
	viewTopnPattern      = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-topn\s*$`)
	viewBracketsPattern  = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-brackets\s*$`)
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
	if setupStatuslineCmdPattern.MatchString(prompt) {
		return handleSetupStatusline()
	}
	// Per-mode commands (no-arg = immediate statusline refresh)
	if viewTreePattern.MatchString(prompt) {
		return handleViewMode(sessionID, "tree")
	}
	if viewIciclePattern.MatchString(prompt) {
		return handleViewMode(sessionID, "icicle")
	}
	if viewCollapsedPattern.MatchString(prompt) {
		return handleViewMode(sessionID, "collapsed")
	}
	if viewSmartPattern.MatchString(prompt) {
		return handleViewMode(sessionID, "smart")
	}
	if viewTopnPattern.MatchString(prompt) {
		return handleViewMode(sessionID, "topn")
	}
	if viewBracketsPattern.MatchString(prompt) {
		return handleViewMode(sessionID, "brackets")
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

	// Persist to config for future sessions
	_ = persistViewModeToConfig(mode)

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

	_ = persistViewModeToConfig(mode)
	blockPrompt(fmt.Sprintf("View: %s", mode))
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

// handleSetupStatusline shows status line setup instructions.
// If no status line configured: shows install path
// If existing status line: shows manual integration snippet
func handleSetupStatusline() int {
	// Get the plugin bin directory from current executable path
	binPath := getBumperLanesBinPath()

	if hasStatusLineConfigured() {
		// Show manual integration snippet for existing status line
		blockPrompt(fmt.Sprintf(`Custom status line detected.

To add bumper-lanes diff tree, append to the END of your script:

  # Bumper-lanes widgets (add after your main status line output)
  BUMPER_LANES="%s"
  bumper_indicator=$(echo "$input" | $BUMPER_LANES status --widget=indicator)
  diff_tree=$(echo "$input" | $BUMPER_LANES status --widget=diff-tree)
  [[ -n "$bumper_indicator" ]] && echo "$bumper_indicator"
  [[ -n "$diff_tree" ]] && echo "$diff_tree"

Note: Your script must capture input=$(cat) at the start.
Status lines can be multi-line: line 1 = status bar, additional lines = widgets.`, binPath))
		return 0
	}

	// No status line - show install instructions
	addonPath := getAddonScriptPath()
	blockPrompt(fmt.Sprintf(`No status line configured.

Option 1: Use the bumper-lanes addon script
Add to ~/.claude/settings.json:

  "statusLine": {
    "command": "%s"
  }

Option 2: Create a custom status line
See: https://github.com/kylesnowschwartz/claude-bumper-lanes#status-line`, addonPath))
	return 0
}

// getBumperLanesBinPath returns the path to the bumper-lanes binary.
func getBumperLanesBinPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "bumper-lanes" // fallback to PATH
	}
	return exe
}

// getAddonScriptPath returns the path to the addon status line script.
func getAddonScriptPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "/path/to/bumper-lanes-plugin/status-lines/bumper-lanes-addon.sh"
	}
	// exe is .../bin/bumper-lanes, addon is .../status-lines/bumper-lanes-addon.sh
	binDir := filepath.Dir(exe)
	pluginDir := filepath.Dir(binDir)
	return filepath.Join(pluginDir, "status-lines", "bumper-lanes-addon.sh")
}

// hasStatusLineConfigured is also used by session_start.go
// (imported from session_start.go)

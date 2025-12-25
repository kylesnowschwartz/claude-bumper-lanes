package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// SessionStart handles the SessionStart hook event.
// It captures the baseline tree and initializes session state.
// Returns exit code: 0 = success, 1 = warning (shows stderr to user).
func SessionStart(input *HookInput) int {
	// Check if this is a git repository
	if !IsGitRepo() {
		return 0 // Fail open - not a git repo
	}

	// Capture baseline tree
	baselineTree, err := CaptureTree()
	if err != nil {
		return 0 // Fail open
	}

	// Get current branch for staleness detection
	baselineBranch := GetCurrentBranch()

	// Load threshold from config
	threshold := config.LoadThreshold()

	// Create and save session state
	sess, err := state.New(input.SessionID, baselineTree, baselineBranch, threshold)
	if err != nil {
		return 0 // Fail open
	}

	if err := sess.Save(); err != nil {
		return 0 // Fail open
	}

	// One-time prompt about status line setup (once per repo)
	if !config.LoadStatusLinePrompted() {
		_ = config.SaveStatusLinePrompted() // Best effort
		fmt.Fprintln(os.Stderr, "[bumper-lanes] Run /bumper-setup-statusline to enable diff tree visualization.")
		return 1 // Exit 1 shows stderr as warning
	}

	return 0
}

// hasStatusLineConfigured checks if ~/.claude/settings.json has statusLine configured.
func hasStatusLineConfigured() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true // Assume configured on error (fail open)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false // No settings file = not configured
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return true // Invalid JSON - fail open
	}

	// Check for statusLine key
	statusLine, ok := settings["statusLine"]
	if !ok {
		return false
	}

	// Check if statusLine has a command configured
	statusLineMap, ok := statusLine.(map[string]interface{})
	if !ok {
		return false
	}

	cmd, ok := statusLineMap["command"]
	if !ok {
		return false
	}

	// Non-empty command = configured
	cmdStr, ok := cmd.(string)
	return ok && cmdStr != ""
}

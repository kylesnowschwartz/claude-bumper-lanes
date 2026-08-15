package statusline

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
)

// binaryFileName is the bumper-lanes binary name.
const binaryFileName = "bumper-lanes"

// EnsureInstalled maintains the bumper-lanes status line setup. bumper-lanes
// never touches a user's own status line command: an existing command is
// left alone (compose the gauge yourself via `bumper-lanes status
// --widget=indicator`). When allowInstall is true and no status line is
// configured, settings point directly at the binary.
// Returns a message to show the user, or empty string if no action needed.
// This is idempotent - checks actual state each time rather than caching.
func EnsureInstalled(log *logging.Logger, allowInstall bool) string {
	// A test process must never touch the user's status line: the repair
	// path compares paths against os.Executable(), and recording a
	// transient go-test build path kills the gauge silently once that
	// binary is cleaned up.
	if logging.IsTestProcess() {
		return ""
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Warn("failed to get home dir for status line setup: %v (failing open)", err)
		return "" // Fail open
	}

	// Get current status line command
	currentCmd := getStatusLineCommand(homeDir)

	// Already pointing at our binary - repair a stale path (plugin updated)
	if isOurBinary(currentCmd) {
		if !isBinaryStale(currentCmd) {
			return "" // Already set up and current
		}
		newBinaryPath, err := os.Executable()
		if err != nil {
			log.Warn("failed to get executable path: %v (failing open)", err)
			return "" // Fail open
		}
		if err := updateSettingsWithJq(homeDir, newBinaryPath); err != nil {
			log.Warn("failed to update settings.json: %v (failing open)", err)
			return "" // Fail open - old binary might still work
		}
		return "[bumper-lanes] Updated status line for new plugin version. Restart session to activate."
	}

	// A foreign status line is the user's own - never touch it. The gauge
	// composes into custom status lines via `status --widget=indicator`.
	if currentCmd != "" {
		return ""
	}

	// Installing from scratch rewrites the user-global settings file - only
	// with explicit opt-in.
	if !allowInstall {
		return ""
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("[bumper-lanes] Couldn't find binary path: %v", err)
	}
	if err := updateSettingsWithJq(homeDir, binaryPath); err != nil {
		return fmt.Sprintf("[bumper-lanes] Couldn't update settings: %v\nManual setup: point statusLine.command in ~/.claude/settings.json at the bumper-lanes binary.", err)
	}
	return "[bumper-lanes] Status line configured! Restart session to see the budget gauge."
}

// getStatusLineCommand reads the current statusLine.command from settings.json.
func getStatusLineCommand(homeDir string) string {
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return ""
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return ""
	}

	statusLine, ok := settings["statusLine"].(map[string]interface{})
	if !ok {
		return ""
	}

	cmd, ok := statusLine["command"].(string)
	if !ok {
		return ""
	}
	return cmd
}

// isOurBinary checks if the given command is our binary directly.
func isOurBinary(cmd string) bool {
	if cmd == "" {
		return false
	}
	return filepath.Base(cmd) == binaryFileName
}

// isBinaryStale checks if the binary path is stale (plugin updated).
// Returns true if the path doesn't match the current executable.
func isBinaryStale(cmd string) bool {
	currentBinaryPath, err := os.Executable()
	if err != nil {
		return false // Can't determine - assume not stale
	}
	return cmd != currentBinaryPath
}

// updateSettingsWithJq updates ~/.claude/settings.json using jq.
func updateSettingsWithJq(homeDir, command string) error {
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	// Check if jq is available
	if _, err := exec.LookPath("jq"); err != nil {
		return fmt.Errorf("jq not installed")
	}

	// Ensure settings.json exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		// Create minimal settings file
		if err := os.WriteFile(settingsPath, []byte("{}"), 0644); err != nil {
			return err
		}
	}

	// Use jq to update settings - must set both type and command
	jqExpr := fmt.Sprintf(`.statusLine.type = "command" | .statusLine.command = %q`, command)
	cmd := exec.Command("jq", jqExpr, settingsPath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("jq failed: %w", err)
	}

	// Write back
	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return err
	}

	return nil
}

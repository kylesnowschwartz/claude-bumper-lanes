// Package statusline provides a complete status line for Claude Code.
// Outputs model, git branch, cost, and bumper-lanes widget.
package statusline

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// StatusInput is the JSON payload from Claude Code's status line hook.
type StatusInput struct {
	SessionID string `json:"session_id"`
	Model     struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	Cost struct {
		TotalCostUSD float64 `json:"total_cost_usd"`
	} `json:"cost"`
}

// StatusOutput holds the widget output.
type StatusOutput struct {
	// StatusLine is the main status text (e.g., "active (125/400 - 31%)")
	StatusLine string
	// DiffTree is the multi-line diff visualization (may be empty)
	DiffTree string
	// State is the bumper-lanes state: "active", "tripped", "paused", or "" (inactive)
	State string
	// Score is the current diff score
	Score int
	// Limit is the threshold limit
	Limit int
	// Percentage is score/limit as integer percentage
	Percentage int
}

// ANSI color codes
const (
	colorGreen   = "\033[32m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[94m"
	colorMagenta = "\033[95m"
	colorCost    = "\033[35m"
	colorReset   = "\033[0m"
)

// Render produces a complete status line from Claude Code's status input.
// Returns StatusOutput with formatted text ready for display.
func Render(input *StatusInput) (*StatusOutput, error) {
	var parts []string

	// Model name
	model := input.Model.DisplayName
	if model == "" {
		model = "?"
	}
	parts = append(parts, fmt.Sprintf("%s[%s]%s", colorMagenta, model, colorReset))

	// Directory name (basename only)
	if input.Workspace.CurrentDir != "" {
		dir := filepath.Base(input.Workspace.CurrentDir)
		parts = append(parts, dir)
	}

	// Change to workspace for git operations
	origDir, _ := os.Getwd()
	if input.Workspace.CurrentDir != "" {
		if err := os.Chdir(input.Workspace.CurrentDir); err == nil {
			defer os.Chdir(origDir)
		}
	}

	// Git branch with dirty indicator
	if branch := getGitBranch(); branch != "" {
		if isGitDirty() {
			parts = append(parts, fmt.Sprintf("%s%s%s %s*%s", colorBlue, branch, colorReset, colorYellow, colorReset))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s%s", colorBlue, branch, colorReset))
		}
	}

	// Cost
	cost := fmt.Sprintf("$%.2f", input.Cost.TotalCostUSD)
	parts = append(parts, fmt.Sprintf("%s%s%s", colorCost, cost, colorReset))

	// Bumper-lanes widget (if active)
	var stateStr string
	var score, limit, percentage int
	var diffTree string

	sess, err := state.Load(input.SessionID)
	if err == nil {
		// Calculate fresh score
		score = calculateScore(sess.BaselineTree)
		limit = sess.ThresholdLimit
		if limit > 0 {
			percentage = (score * 100) / limit
		}

		// Determine state
		if sess.Paused {
			stateStr = "paused"
		} else if sess.StopTriggered {
			stateStr = "tripped"
		} else {
			stateStr = "active"
		}

		// Add bumper status to parts
		parts = append(parts, formatBumperStatus(stateStr, score, limit, percentage))

		// Get diff tree visualization (show even when paused)
		viewMode := sess.GetViewMode()
		if viewMode == "" {
			viewMode = config.LoadViewMode()
		}
		viewOpts := sess.GetViewOpts()
		diffTree = getDiffTree(viewMode, viewOpts)
	}

	return &StatusOutput{
		StatusLine: strings.Join(parts, " | "),
		DiffTree:   diffTree,
		State:      stateStr,
		Score:      score,
		Limit:      limit,
		Percentage: percentage,
	}, nil
}

// getGitBranch returns current branch name or empty string.
func getGitBranch() string {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// isGitDirty returns true if working tree has uncommitted changes.
func isGitDirty() bool {
	cmd := exec.Command("git", "diff", "--quiet", "HEAD")
	err := cmd.Run()
	return err != nil // non-zero exit = dirty
}

// formatBumperStatus produces the colored bumper-lanes status text.
func formatBumperStatus(stateStr string, score, limit, percentage int) string {
	switch stateStr {
	case "paused":
		return fmt.Sprintf("%sPaused: /bumper-resume%s", colorYellow, colorReset)
	case "tripped":
		return fmt.Sprintf("%stripped (%d/%d - %d%%)%s",
			colorRed, score, limit, percentage, colorReset)
	default: // active
		return fmt.Sprintf("%sactive (%d/%d - %d%%)%s",
			colorGreen, score, limit, percentage, colorReset)
	}
}

// calculateScore runs diff-viz to get stats, then calculates score locally.
// This keeps scoring logic in bumper-lanes (policy) while diff-viz provides raw data.
func calculateScore(baselineTree string) int {
	if baselineTree == "" {
		return 0
	}

	// Find diff-viz binary relative to bumper-lanes binary
	binPath := findDiffVizBinary()
	if binPath == "" {
		return 0
	}

	cmd := exec.Command(binPath, "--stats-json", "--baseline="+baselineTree)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Parse raw stats from diff-viz
	var stats scoring.StatsJSON
	if err := json.Unmarshal(output, &stats); err != nil {
		return 0
	}

	// Calculate score using bumper-lanes scoring policy
	result := scoring.Calculate(&stats)
	return result.Score
}

// getDiffTree runs diff-viz to get the tree visualization.
// viewOpts contains additional flags like "--width 100 --depth 3".
func getDiffTree(viewMode, viewOpts string) string {
	binPath := findDiffVizBinary()
	if binPath == "" {
		return ""
	}

	if viewMode == "" {
		viewMode = "tree"
	}

	// Build args: mode + any additional options
	args := []string{"--mode=" + viewMode}
	if viewOpts != "" {
		// Split opts string into individual args
		for _, opt := range strings.Fields(viewOpts) {
			args = append(args, opt)
		}
	}

	cmd := exec.Command(binPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Only trim trailing whitespace - preserve leading spaces for alignment
	result := strings.TrimRight(string(output), " \t\n\r")
	if result == "No changes" {
		return ""
	}
	return result
}

// findDiffVizBinary locates the git-diff-tree-go binary.
// Looks in: same directory as this binary, then PATH.
func findDiffVizBinary() string {
	// Try same directory as current executable
	exe, err := os.Executable()
	if err == nil {
		binDir := filepath.Dir(exe)
		candidate := filepath.Join(binDir, "git-diff-tree-go")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fall back to PATH
	path, err := exec.LookPath("git-diff-tree-go")
	if err == nil {
		return path
	}

	return ""
}

// ParseInput parses JSON input from stdin.
func ParseInput(data []byte) (*StatusInput, error) {
	var input StatusInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing status input: %w", err)
	}
	return &input, nil
}

// FormatOutput converts StatusOutput to the final string output.
// Applies non-breaking space conversion for Claude Code compatibility.
func FormatOutput(out *StatusOutput) string {
	if out.StatusLine == "" {
		return ""
	}

	var result strings.Builder
	result.WriteString(out.StatusLine)
	result.WriteString("\n")

	if out.DiffTree != "" {
		// Convert spaces to non-breaking spaces and prepend ANSI reset per line
		// (ccstatusline technique for preserving whitespace in Claude Code)
		lines := strings.Split(out.DiffTree, "\n")
		for _, line := range lines {
			// Replace spaces with non-breaking space (U+00A0)
			line = strings.ReplaceAll(line, " ", "\u00A0")
			result.WriteString("\033[0m")
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String()
}

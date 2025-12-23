// Package statusline provides the bumper-lanes status line widget.
// This outputs formatted status text that can be integrated into any status line.
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
// We only parse the fields we need.
type StatusInput struct {
	SessionID string `json:"session_id"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
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
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// Render produces the bumper-lanes status widget from Claude Code's status input.
// Returns StatusOutput with formatted text ready for display.
func Render(input *StatusInput) (*StatusOutput, error) {
	if input.Workspace.CurrentDir == "" {
		return &StatusOutput{}, nil // Not in a workspace, nothing to show
	}

	// Change to workspace directory for git operations
	origDir, _ := os.Getwd()
	if err := os.Chdir(input.Workspace.CurrentDir); err != nil {
		return &StatusOutput{}, nil // Can't access workspace
	}
	defer os.Chdir(origDir)

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		return &StatusOutput{}, nil // No session = bumper-lanes not active
	}

	// Calculate fresh score using diff-viz binary
	score := calculateScore(sess.BaselineTree)
	limit := sess.ThresholdLimit
	percentage := 0
	if limit > 0 {
		percentage = (score * 100) / limit
	}

	// Determine state
	var stateStr string
	if sess.Paused {
		stateStr = "paused"
	} else if sess.StopTriggered {
		stateStr = "tripped"
	} else {
		stateStr = "active"
	}

	// Format status line
	statusLine := formatStatusLine(stateStr, score, limit, percentage)

	// Get diff tree visualization
	// Priority: session state > personal config > repo config > default
	diffTree := ""
	if stateStr != "paused" {
		viewMode := sess.GetViewMode()
		if viewMode == "" {
			viewMode = config.LoadViewMode() // fallback chain
		}
		diffTree = getDiffTree(viewMode)
	}

	return &StatusOutput{
		StatusLine: statusLine,
		DiffTree:   diffTree,
		State:      stateStr,
		Score:      score,
		Limit:      limit,
		Percentage: percentage,
	}, nil
}

// formatStatusLine produces the colored status text.
func formatStatusLine(stateStr string, score, limit, percentage int) string {
	switch stateStr {
	case "paused":
		return fmt.Sprintf("%sPaused: run /bumper-resume%s", colorYellow, colorReset)
	case "tripped":
		return fmt.Sprintf("%sbumper-lanes tripped (%d/%d - %d%%)%s",
			colorRed, score, limit, percentage, colorReset)
	default: // active
		return fmt.Sprintf("%sbumper-lanes active (%d/%d - %d%%)%s",
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
func getDiffTree(viewMode string) string {
	binPath := findDiffVizBinary()
	if binPath == "" {
		return ""
	}

	if viewMode == "" {
		viewMode = "tree"
	}

	cmd := exec.Command(binPath, "--mode="+viewMode)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	result := strings.TrimSpace(string(output))
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

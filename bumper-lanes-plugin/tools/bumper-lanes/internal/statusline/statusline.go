// Package statusline provides a status line for Claude Code.
// Outputs model, git branch, cost, and the one-line bumper-lanes indicator.
// Rich diff visualizations live in the diff-viz CLI and the on-demand
// /bumper-diff command, not in this ambient surface.
package statusline

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
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
	// StatusLine is the full status text (model | dir | branch | cost | bumper)
	StatusLine string
	// BumperIndicator is just the bumper-lanes piece (e.g., "▂ 31%")
	BumperIndicator string
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
// The bumper indicator uses the cached session score: it is recomputed from
// the baseline diff on every Write/Edit and Stop, which keeps the line
// truthful without paying a working-tree capture (~60-110ms) per refresh.
func Render(input *StatusInput) (*StatusOutput, error) {
	start := time.Now()
	log := logging.New(input.SessionID, "statusline")

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
	out := renderBumperWidget(input.SessionID)
	if out.BumperIndicator != "" {
		parts = append(parts, out.BumperIndicator)
	}
	out.StatusLine = strings.Join(parts, " | ")

	log.Debug("render completed in %v", time.Since(start))
	return out, nil
}

// RenderIndicator produces only the bumper gauge from cached session state.
// This is the path generated statusline wrappers call per refresh: it skips
// the model/branch/cost widgets, so the only git subprocess is the git-dir
// lookup inside state.Load.
func RenderIndicator(input *StatusInput) (*StatusOutput, error) {
	if input.Workspace.CurrentDir != "" {
		origDir, _ := os.Getwd()
		if err := os.Chdir(input.Workspace.CurrentDir); err == nil {
			defer os.Chdir(origDir)
		}
	}
	return renderBumperWidget(input.SessionID), nil
}

// renderBumperWidget formats the gauge from cached session state only.
func renderBumperWidget(sessionID string) *StatusOutput {
	out := &StatusOutput{}

	sess, err := state.Load(sessionID)
	if err != nil {
		return out
	}

	out.Score = sess.Score
	out.Limit = sess.ThresholdLimit
	if out.Limit > 0 {
		out.Percentage = (out.Score * 100) / out.Limit
	}

	// Determine state
	if sess.ThresholdLimit == 0 {
		out.State = "disabled"
	} else if sess.Paused {
		out.State = "paused"
	} else if sess.StopTriggered {
		out.State = "tripped"
	} else {
		out.State = "active"
	}

	out.BumperIndicator = formatBumperStatus(out.State, out.Percentage, len(sess.Tripwires) > 0, sess.NetLines)
	return out
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

// formatBumperStatus produces the one-line bumper-lanes indicator.
// Progressive traffic light: ▂ green <70%, ▂▄ +yellow 70-90%, ▂▄█ +red >90%
// or tripped. A red ⚠ marks tripwire hits in the current increment. A
// net-negative increment (tree shrank) is shown in green: subtraction is
// rewarded with visibility, never with spendable budget.
func formatBumperStatus(stateStr string, percentage int, hasTripwires bool, netLines int) string {
	tripwireGlyph := ""
	if hasTripwires {
		tripwireGlyph = fmt.Sprintf(" %s⚠%s", colorRed, colorReset)
	}
	netGlyph := ""
	if netLines < 0 {
		netGlyph = fmt.Sprintf(" %s%d lines%s", colorGreen, netLines, colorReset)
	}

	// Disabled state shows text in blue
	if stateStr == "disabled" {
		return fmt.Sprintf("%sDisabled%s", colorBlue, colorReset)
	}

	// Paused state shows text instead of bar
	if stateStr == "paused" {
		return fmt.Sprintf("%sPaused%s%s", colorYellow, colorReset, tripwireGlyph)
	}

	bar := formatTrafficLightBar(percentage, stateStr == "tripped")
	return bar + tripwireGlyph + netGlyph
}

// formatTrafficLightBar returns a colored traffic light gauge with percentage.
// Progressive reveal: green <70%, green+yellow 70-90%, all three >90% or tripped.
// Uses increasing height blocks: ▂ (short), ▄ (medium), █ (tall).
func formatTrafficLightBar(percentage int, tripped bool) string {
	// Unicode block characters of increasing height
	const (
		shortBar  = "▂" // U+2582 - lower quarter block (green zone)
		mediumBar = "▄" // U+2584 - lower half block (yellow zone)
		tallBar   = "█" // U+2588 - full block (red zone)
	)

	var bar string

	switch {
	case tripped || percentage >= 90:
		// Red zone: show all three bars
		bar = fmt.Sprintf("%s%s%s%s%s%s%s",
			colorGreen, shortBar,
			colorYellow, mediumBar,
			colorRed, tallBar,
			colorReset)
	case percentage >= 70:
		// Yellow zone: show green + yellow
		bar = fmt.Sprintf("%s%s%s%s%s",
			colorGreen, shortBar,
			colorYellow, mediumBar,
			colorReset)
	default:
		// Green zone: show only green
		bar = fmt.Sprintf("%s%s%s", colorGreen, shortBar, colorReset)
	}

	return fmt.Sprintf("%s %d%%", bar, percentage)
}

// ParseInput parses JSON input from stdin.
func ParseInput(data []byte) (*StatusInput, error) {
	var input StatusInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing status input: %w", err)
	}
	return &input, nil
}

// Widget types for selective output.
const (
	WidgetAll       = "all"       // Full status line (default)
	WidgetIndicator = "indicator" // Just the bumper-lanes indicator
)

// FormatOutput converts StatusOutput to the final string output.
// Widget selects which component to output: "all" or "indicator".
// "diff-tree" is accepted for wrappers generated before v4 and returns
// nothing: the visualization moved to the /bumper-diff command.
func FormatOutput(out *StatusOutput, widget string) string {
	switch widget {
	case WidgetIndicator:
		return out.FormatIndicator()
	case "diff-tree":
		return ""
	default:
		return out.FormatAll()
	}
}

// FormatIndicator returns just the bumper-lanes indicator (e.g., "▂ 31%").
func (out *StatusOutput) FormatIndicator() string {
	if out.BumperIndicator == "" {
		return ""
	}
	return out.BumperIndicator + "\n"
}

// FormatAll returns the full status line.
func (out *StatusOutput) FormatAll() string {
	if out.StatusLine == "" {
		return ""
	}
	return out.StatusLine + "\n"
}

// Package statusline provides a complete status line for Claude Code.
// Outputs model, git branch, cost, and bumper-lanes widget.
package statusline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/diff-viz/v2/diff"
	"github.com/kylesnowschwartz/diff-viz/v2/render"

	diffvizconfig "github.com/kylesnowschwartz/diff-viz/v2/config"
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
	// BumperIndicator is just the bumper-lanes piece (e.g., "active (125/400 - 31%)")
	BumperIndicator string
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
	var bumperIndicator string

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

		// Get view mode (needed for both indicator and diff tree)
		viewMode := sess.GetViewMode()
		if viewMode == "" {
			viewMode = config.LoadViewMode()
		}

		// Format bumper indicator (capture for both full line and standalone use)
		// viewMode included to force status line refresh when mode changes
		bumperIndicator = formatBumperStatus(stateStr, score, limit, percentage, viewMode)
		parts = append(parts, bumperIndicator)

		// Get diff tree visualization (show even when paused)
		viewOpts := sess.GetViewOpts()
		diffTree = getDiffTree(viewMode, viewOpts)
	}

	return &StatusOutput{
		StatusLine:      strings.Join(parts, " | "),
		BumperIndicator: bumperIndicator,
		DiffTree:        diffTree,
		State:           stateStr,
		Score:           score,
		Limit:           limit,
		Percentage:      percentage,
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

// formatBumperStatus produces a traffic light bar for bumper-lanes status.
// Bar color: green <60%, yellow 60-80%, red >80% or tripped.
// viewMode is included to force status line refresh when mode changes.
func formatBumperStatus(stateStr string, score, limit, percentage int, viewMode string) string {
	if viewMode == "" {
		viewMode = "tree"
	}

	// Paused state shows text instead of bar
	if stateStr == "paused" {
		return fmt.Sprintf("%sPaused%s [%s]", colorYellow, colorReset, viewMode)
	}

	// Build 5-char traffic light bar
	bar := formatTrafficLightBar(percentage, stateStr == "tripped")

	return fmt.Sprintf("%s [%s]", bar, viewMode)
}

// formatTrafficLightBar returns a 5-char colored bar showing percentage.
// Color tiers: green <70%, yellow 70-90%, red >90%.
func formatTrafficLightBar(percentage int, tripped bool) string {
	// Determine fill level (0-5 chars)
	fill := (percentage * 5) / 100
	if fill > 5 {
		fill = 5
	}
	if fill < 0 {
		fill = 0
	}

	// Determine color based on tier (or tripped state)
	var color string
	switch {
	case tripped:
		color = colorRed
	case percentage >= 90:
		color = colorRed
	case percentage >= 70:
		color = colorYellow
	default:
		color = colorGreen
	}

	// Build bar: filled portion + empty portion
	filled := strings.Repeat("█", fill)
	empty := strings.Repeat("░", 5-fill)

	return fmt.Sprintf("%s%s%s%s", color, filled, empty, colorReset)
}

// calculateScore uses diff-viz library to get stats, then calculates score.
// This keeps scoring logic in bumper-lanes (policy) while diff-viz provides raw data.
func calculateScore(baselineTree string) int {
	if baselineTree == "" {
		return 0
	}

	// Capture current working tree
	currentTree, err := diff.CaptureCurrentTree()
	if err != nil {
		return 0
	}

	// Get diff stats from baseline to current
	stats, _, err := diff.GetTreeDiffStats(baselineTree, currentTree)
	if err != nil {
		return 0
	}

	// Calculate score using bumper-lanes scoring policy
	jsonStats := stats.ToJSON()
	result := scoring.Calculate(&jsonStats)
	return result.Score
}

// getDiffTree uses diff-viz library to render the tree visualization.
// Uses diff-viz config system for per-mode defaults from .bumper-lanes.json.
func getDiffTree(viewMode, viewOpts string) string {
	if viewMode == "" {
		viewMode = "tree"
	}

	// Get current diff stats (working tree vs HEAD)
	stats, _, err := diff.GetAllStats()
	if err != nil || stats.TotalFiles == 0 {
		return ""
	}

	// Load diff-viz config from .bumper-lanes.json (ignores bumper-specific fields)
	configPath := config.GetConfigPath()
	cfg, _ := diffvizconfig.Load(configPath) // nil cfg is fine, Resolve handles it

	// Parse CLI-style overrides from viewOpts (legacy support)
	var cliFlags *diffvizconfig.ModeConfig
	if viewOpts != "" {
		cliFlags = &diffvizconfig.ModeConfig{}
		for _, opt := range strings.Fields(viewOpts) {
			if strings.HasPrefix(opt, "--width=") {
				var w int
				fmt.Sscanf(opt, "--width=%d", &w)
				cliFlags.Width = &w
			} else if strings.HasPrefix(opt, "--depth=") {
				var d int
				fmt.Sscanf(opt, "--depth=%d", &d)
				cliFlags.Depth = &d
			} else if strings.HasPrefix(opt, "--expand=") {
				var e int
				fmt.Sscanf(opt, "--expand=%d", &e)
				cliFlags.Expand = &e
			}
		}
	}

	// Resolve config: global defaults < mode defaults < config file < CLI flags
	resolved := cfg.Resolve(viewMode, cliFlags)

	// Render to buffer
	var buf bytes.Buffer
	useColor := true
	renderer := getRenderer(viewMode, &buf, useColor, resolved)
	renderer.Render(stats)

	// Trim trailing whitespace, preserve leading
	result := strings.TrimRight(buf.String(), " \t\n\r")
	if result == "No changes" {
		return ""
	}
	return result
}

// diffRenderer is a local interface matching diff-viz's renderer pattern.
type diffRenderer interface {
	Render(stats *diff.DiffStats)
}

// getRenderer returns the appropriate renderer for the given mode.
// Uses resolved config from diff-viz config system for per-mode settings.
func getRenderer(mode string, buf *bytes.Buffer, useColor bool, cfg diffvizconfig.ResolvedConfig) diffRenderer {
	switch mode {
	case "tree":
		return render.NewTreeRenderer(buf, useColor)
	case "smart":
		r := render.NewSmartSparklineRenderer(buf, useColor)
		r.Width = cfg.Width
		r.MaxDepth = cfg.Depth
		return r
	case "sparkline-tree":
		r := render.NewSparklineTreeRenderer(buf, useColor)
		r.MaxDepth = cfg.Depth
		r.N = cfg.N
		return r
	case "hotpath":
		r := render.NewHotpathRenderer(buf, useColor)
		r.MaxDepth = cfg.Depth
		return r
	case "icicle":
		r := render.NewIcicleRenderer(buf, useColor)
		r.Width = cfg.Width
		r.MaxDepth = cfg.Depth
		return r
	case "brackets":
		r := render.NewBracketsRenderer(buf, useColor)
		r.Width = cfg.Width
		r.ExpandDepth = cfg.Expand
		return r
	case "gauge":
		r := render.NewGaugeRenderer(buf, useColor)
		r.Width = cfg.Width
		return r
	case "depth":
		r := render.NewDepthRenderer(buf, useColor)
		r.MaxDepth = cfg.Depth
		r.Width = cfg.Width
		return r
	case "heatmap":
		r := render.NewHeatmapRenderer(buf, useColor)
		r.MaxDepth = cfg.Depth
		return r
	case "stat":
		return render.NewStatRenderer(buf, nil)
	default:
		return render.NewTreeRenderer(buf, useColor)
	}
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
	WidgetAll       = "all"       // Full status line + diff tree (default)
	WidgetIndicator = "indicator" // Just the bumper-lanes indicator
	WidgetDiffTree  = "diff-tree" // Just the diff visualization
)

// FormatOutput converts StatusOutput to the final string output.
// Widget selects which component to output: "all", "indicator", or "diff-tree".
// Applies non-breaking space conversion for Claude Code compatibility.
func FormatOutput(out *StatusOutput, widget string) string {
	switch widget {
	case WidgetIndicator:
		return out.FormatIndicator()
	case WidgetDiffTree:
		return out.FormatDiffTree()
	default:
		return out.FormatAll()
	}
}

// FormatIndicator returns just the bumper-lanes indicator (e.g., "active (125/400 - 31%)").
func (out *StatusOutput) FormatIndicator() string {
	if out.BumperIndicator == "" {
		return ""
	}
	return out.BumperIndicator + "\n"
}

// FormatDiffTree returns just the diff visualization with non-breaking space handling.
func (out *StatusOutput) FormatDiffTree() string {
	if out.DiffTree == "" {
		return ""
	}
	return formatDiffTreeLines(out.DiffTree)
}

// FormatAll returns the full status line plus diff tree.
func (out *StatusOutput) FormatAll() string {
	if out.StatusLine == "" {
		return ""
	}

	var result strings.Builder
	result.WriteString(out.StatusLine)
	result.WriteString("\n")

	if out.DiffTree != "" {
		result.WriteString(formatDiffTreeLines(out.DiffTree))
	}

	return result.String()
}

// formatDiffTreeLines applies non-breaking space conversion for Claude Code compatibility.
func formatDiffTreeLines(diffTree string) string {
	var result strings.Builder
	lines := strings.Split(diffTree, "\n")
	for _, line := range lines {
		// Replace spaces with non-breaking space (U+00A0)
		line = strings.ReplaceAll(line, " ", "\u00A0")
		result.WriteString("\033[0m")
		result.WriteString(line)
		result.WriteString("\n")
	}
	return result.String()
}

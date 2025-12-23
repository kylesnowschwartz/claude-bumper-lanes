// Command git-diff-tree displays hierarchical diff visualization.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/render"
)

// validModes is the single source of truth for available visualization modes.
// Add new modes here - they'll automatically appear in help and validation.
var validModes = []string{"tree", "collapsed", "smart", "topn", "icicle", "brackets"}

// modeDescriptions provides help text for each mode.
var modeDescriptions = map[string]string{
	"tree":      "Indented tree with file stats (default)",
	"collapsed": "Single-line summary per directory",
	"smart":     "Depth-2 aggregated sparkline",
	"topn":      "Top N files by change size (hotspots)",
	"icicle":    "Horizontal icicle chart (width = magnitude)",
	"brackets":  "Nested brackets [dir file█ file██] (single-line hierarchy)",
}

func usage() string {
	var sb strings.Builder
	sb.WriteString(`git-diff-tree - Hierarchical diff visualization

Usage:
  git-diff-tree [flags] [<commit> [<commit>]]

Examples:
  git-diff-tree                    Working tree vs HEAD
  git-diff-tree --cached           Staged changes only
  git-diff-tree HEAD~3             Last 3 commits
  git-diff-tree main feature       Compare branches
  git-diff-tree -m smart           Compact sparkline view
  git-diff-tree --stats-json       Output raw diff stats as JSON

Modes:
`)
	for _, mode := range validModes {
		sb.WriteString(fmt.Sprintf("  %-10s %s\n", mode, modeDescriptions[mode]))
	}
	sb.WriteString("\nFlags:\n")
	return sb.String()
}

// Renderer interface for diff output.
type Renderer interface {
	Render(stats *diff.DiffStats)
}

func main() {
	// Custom usage
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage())
		flag.PrintDefaults()
	}

	// Parse flags
	mode := flag.String("m", "tree", "Output mode (shorthand)")
	modeLong := flag.String("mode", "tree", "Output mode: "+strings.Join(validModes, ", "))
	noColor := flag.Bool("no-color", false, "Disable color output")
	width := flag.Int("width", 100, "Output width in columns (for icicle mode)")
	depth := flag.Int("depth", 4, "Max hierarchy depth to render (for icicle mode, 0=unlimited)")
	help := flag.Bool("h", false, "Show help")
	listModes := flag.Bool("list-modes", false, "List valid modes (for scripting)")
	statsJSON := flag.Bool("stats-json", false, "Output raw diff stats as JSON (for bumper-lanes)")
	baseline := flag.String("baseline", "", "Baseline tree SHA to compare against (uses current working tree)")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *listModes {
		fmt.Println(strings.Join(validModes, " "))
		os.Exit(0)
	}

	// Handle --stats-json mode (raw stats for bumper-lanes)
	if *statsJSON {
		outputStatsJSON(*baseline)
		return
	}

	// Use -m if set, otherwise --mode
	selectedMode := *modeLong
	if *mode != "tree" {
		selectedMode = *mode
	}

	// Validate mode
	if !isValidMode(selectedMode) {
		fmt.Fprintf(os.Stderr, "unknown mode: %s (valid: %s)\n", selectedMode, strings.Join(validModes, ", "))
		os.Exit(1)
	}

	// Get diff stats with remaining args
	stats, err := diff.GetAllStats(flag.Args()...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	useColor := !*noColor

	// Select renderer based on mode
	renderer := getRenderer(selectedMode, useColor, *width, *depth)
	renderer.Render(stats)
}

// outputStatsJSON outputs raw diff stats as JSON.
// This provides the interface for bumper-lanes to consume diff data
// without Go import coupling.
func outputStatsJSON(baseline string) {
	var stats *diff.DiffStats
	var err error

	if baseline != "" {
		currentTree, err := diff.CaptureCurrentTree()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error capturing tree: %v\n", err)
			os.Exit(1)
		}
		stats, err = diff.GetTreeDiffStats(baseline, currentTree)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		stats, err = diff.GetAllStats()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	output, err := json.Marshal(stats.ToJSON())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}

func isValidMode(mode string) bool {
	for _, m := range validModes {
		if m == mode {
			return true
		}
	}
	return false
}

func getRenderer(mode string, useColor bool, width, depth int) Renderer {
	switch mode {
	case "tree":
		return render.NewTreeRenderer(os.Stdout, useColor)
	case "collapsed":
		return render.NewCollapsedRenderer(os.Stdout, useColor)
	case "smart":
		return render.NewSmartSparklineRenderer(os.Stdout, useColor)
	case "topn":
		return render.NewTopNRenderer(os.Stdout, useColor, 5)
	case "icicle":
		r := render.NewIcicleRenderer(os.Stdout, useColor)
		r.Width = width
		r.MaxDepth = depth
		return r
	case "brackets":
		return render.NewBracketsRenderer(os.Stdout, useColor)
	default:
		// Should never reach here if isValidMode was called first
		return render.NewTreeRenderer(os.Stdout, useColor)
	}
}

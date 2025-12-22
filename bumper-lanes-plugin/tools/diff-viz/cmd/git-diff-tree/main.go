// Command git-diff-tree displays hierarchical diff visualization.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/render"
)

// validModes is the single source of truth for available visualization modes.
// Add new modes here - they'll automatically appear in help and validation.
var validModes = []string{"tree", "collapsed", "smart", "hier", "stacked", "topn"}

// modeDescriptions provides help text for each mode.
var modeDescriptions = map[string]string{
	"tree":      "Indented tree with file stats (default)",
	"collapsed": "Single-line summary per directory",
	"smart":     "Depth-2 aggregated sparkline",
	"hier":      "Hierarchical depth sparkline",
	"stacked":   "Multi-line stacked bars",
	"topn":      "Top N files by change size (hotspots)",
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
	help := flag.Bool("h", false, "Show help")
	listModes := flag.Bool("list-modes", false, "List valid modes (for scripting)")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *listModes {
		fmt.Println(strings.Join(validModes, " "))
		os.Exit(0)
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
	renderer := getRenderer(selectedMode, useColor)
	renderer.Render(stats)
}

func isValidMode(mode string) bool {
	for _, m := range validModes {
		if m == mode {
			return true
		}
	}
	return false
}

func getRenderer(mode string, useColor bool) Renderer {
	switch mode {
	case "tree":
		return render.NewTreeRenderer(os.Stdout, useColor)
	case "collapsed":
		return render.NewCollapsedRenderer(os.Stdout, useColor)
	case "smart":
		return render.NewSmartSparklineRenderer(os.Stdout, useColor)
	case "hier":
		return render.NewHierarchicalSparklineRenderer(os.Stdout, useColor)
	case "stacked":
		return render.NewStackedSparklineRenderer(os.Stdout, useColor)
	case "topn":
		return render.NewTopNRenderer(os.Stdout, useColor, 5)
	default:
		// Should never reach here if isValidMode was called first
		return render.NewTreeRenderer(os.Stdout, useColor)
	}
}

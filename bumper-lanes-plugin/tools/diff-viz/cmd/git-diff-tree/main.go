// Command git-diff-tree displays hierarchical diff visualization.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/render"
)

const usage = `git-diff-tree - Hierarchical diff visualization

Usage:
  git-diff-tree [flags] [<commit> [<commit>]]

Examples:
  git-diff-tree                    Working tree vs HEAD
  git-diff-tree --cached           Staged changes only
  git-diff-tree HEAD~3             Last 3 commits
  git-diff-tree main feature       Compare branches
  git-diff-tree -m smart           Compact sparkline view

Modes:
  tree       Indented tree with file stats (default)
  collapsed  Single-line summary per directory
  smart      Depth-2 aggregated sparkline
  hier       Hierarchical depth sparkline
  stacked    Multi-line stacked bars

Flags:
`

// Renderer interface for diff output.
type Renderer interface {
	Render(stats *diff.DiffStats)
}

func main() {
	// Custom usage
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}

	// Parse flags
	mode := flag.String("m", "tree", "Output mode (shorthand)")
	modeLong := flag.String("mode", "tree", "Output mode: tree, collapsed, smart, hier, stacked")
	noColor := flag.Bool("no-color", false, "Disable color output")
	help := flag.Bool("h", false, "Show help")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Use -m if set, otherwise --mode
	selectedMode := *modeLong
	if *mode != "tree" {
		selectedMode = *mode
	}

	// Get diff stats with remaining args
	stats, err := diff.GetAllStats(flag.Args()...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	useColor := !*noColor

	// Select renderer based on mode
	var renderer Renderer
	switch selectedMode {
	case "tree":
		renderer = render.NewTreeRenderer(os.Stdout, useColor)
	case "collapsed":
		renderer = render.NewCollapsedRenderer(os.Stdout, useColor)
	case "smart":
		renderer = render.NewSmartSparklineRenderer(os.Stdout, useColor)
	case "hier":
		renderer = render.NewHierarchicalSparklineRenderer(os.Stdout, useColor)
	case "stacked":
		renderer = render.NewStackedSparklineRenderer(os.Stdout, useColor)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s (use tree, collapsed, smart, hier, or stacked)\n", selectedMode)
		os.Exit(1)
	}

	renderer.Render(stats)
}

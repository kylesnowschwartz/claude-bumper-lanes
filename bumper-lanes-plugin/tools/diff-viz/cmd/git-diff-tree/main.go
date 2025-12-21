// Command git-diff-tree displays hierarchical diff visualization.
//
// Usage:
//
//	git-diff-tree                    # Working tree vs HEAD (tree mode)
//	git-diff-tree --cached           # Staged changes
//	git-diff-tree --mode=collapsed   # Compact single-line format
//	git-diff-tree main feature       # Compare branches
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/render"
)

// Renderer interface for diff output.
type Renderer interface {
	Render(stats *diff.DiffStats)
}

func main() {
	// Parse flags
	mode := flag.String("mode", "tree", "Output mode: tree, collapsed, smart, hier, stacked")
	noColor := flag.Bool("no-color", false, "Disable color output")
	flag.Parse()

	// Get diff stats with remaining args
	stats, err := diff.GetAllStats(flag.Args()...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	useColor := !*noColor

	// Select renderer based on mode
	var renderer Renderer
	switch *mode {
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
		fmt.Fprintf(os.Stderr, "unknown mode: %s (use tree, collapsed, sparkline, hier, or stacked)\n", *mode)
		os.Exit(1)
	}

	renderer.Render(stats)
}

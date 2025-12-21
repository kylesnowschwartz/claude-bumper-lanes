// Command git-diff-tree displays hierarchical diff visualization.
//
// Usage:
//
//	git-diff-tree              # Working tree vs HEAD
//	git-diff-tree --cached     # Staged changes
//	git-diff-tree main feature # Compare branches
package main

import (
	"fmt"
	"os"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/render"
)

func main() {
	stats, err := diff.GetAllStats(os.Args[1:]...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	renderer := render.NewTreeRenderer(os.Stdout, true)
	renderer.Render(stats)
}

// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// Height blocks for stacked sparkline (8 levels)
var stackedBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

const (
	stackedMaxDepth = 8 // Max depth levels to show
	stackedBarWidth = 8 // Width of depth histogram
)

// DepthStats holds change magnitude at each depth level.
type DepthStats struct {
	Name      string
	ByDepth   []int // Changes at each depth level (index 0 = root)
	Total     int
	MaxDepth  int
	FileCount int
	HasNew    bool
}

// StackedSparklineRenderer renders diff stats as multi-row depth histograms.
// Format:
//
//	bumper ▁▃▅██▅▃▁
//	tests  ▂
type StackedSparklineRenderer struct {
	UseColor bool
	w        io.Writer
}

// NewStackedSparklineRenderer creates a stacked sparkline renderer.
func NewStackedSparklineRenderer(w io.Writer, useColor bool) *StackedSparklineRenderer {
	return &StackedSparklineRenderer{UseColor: useColor, w: w}
}

// Render outputs diff stats as depth-based sparklines.
func (r *StackedSparklineRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Aggregate by top-level directory with depth breakdown
	dirs := aggregateByDepth(stats.Files)

	// Sort by total changes descending
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Total > dirs[j].Total
	})

	// Find max at any depth for scaling
	maxAtDepth := 0
	for _, d := range dirs {
		for _, v := range d.ByDepth {
			if v > maxAtDepth {
				maxAtDepth = v
			}
		}
	}

	// Find longest name for alignment
	maxNameLen := 0
	for _, d := range dirs {
		if len(d.Name) > maxNameLen {
			maxNameLen = len(d.Name)
		}
	}

	// Render each directory on its own line
	for _, d := range dirs {
		r.renderDir(d, maxAtDepth, maxNameLen)
	}
}

// renderDir renders a single directory's depth histogram.
func (r *StackedSparklineRenderer) renderDir(d DepthStats, maxAtDepth, maxNameLen int) {
	nameColor := ColorDir
	if d.HasNew {
		nameColor = ColorNew
	}

	// Padded name
	fmt.Fprintf(r.w, "%s%-*s%s ", r.color(nameColor), maxNameLen, d.Name, r.color(ColorReset))

	// Depth histogram
	for i := 0; i < stackedBarWidth; i++ {
		changes := 0
		if i < len(d.ByDepth) {
			changes = d.ByDepth[i]
		}

		// Scale to block height (0-8)
		height := 0
		if maxAtDepth > 0 && changes > 0 {
			height = (changes * 8) / maxAtDepth
			if height == 0 {
				height = 1 // At least 1 if there are changes
			}
		}

		// Color based on depth: shallow=green, deep=yellow
		blockColor := ColorAdd
		if i >= 3 {
			blockColor = ColorNew
		}

		fmt.Fprintf(r.w, "%s%s%s", r.color(blockColor), stackedBlocks[height], r.color(ColorReset))
	}

	// Show total
	fmt.Fprintf(r.w, " %s+%d%s", r.color(ColorAdd), d.Total, r.color(ColorReset))

	fmt.Fprintln(r.w)
}

// color returns the ANSI code if color is enabled.
func (r *StackedSparklineRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

// aggregateByDepth groups files by top-level directory,
// tracking changes at each depth level.
func aggregateByDepth(files []diff.FileStat) []DepthStats {
	dirMap := make(map[string]*DepthStats)

	for _, f := range files {
		topDir := getTopDir(f.Path)
		depth := strings.Count(f.Path, "/")
		if depth >= stackedMaxDepth {
			depth = stackedMaxDepth - 1
		}

		if _, ok := dirMap[topDir]; !ok {
			dirMap[topDir] = &DepthStats{
				Name:    topDir,
				ByDepth: make([]int, stackedMaxDepth),
			}
		}
		d := dirMap[topDir]

		changes := f.Additions + f.Deletions
		d.ByDepth[depth] += changes
		d.Total += changes
		d.FileCount++

		if depth > d.MaxDepth {
			d.MaxDepth = depth
		}
		if f.IsUntracked {
			d.HasNew = true
		}
	}

	// Convert to slice
	result := make([]DepthStats, 0, len(dirMap))
	for _, d := range dirMap {
		result = append(result, *d)
	}
	return result
}

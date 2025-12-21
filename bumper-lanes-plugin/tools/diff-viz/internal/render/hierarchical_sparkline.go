// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

const (
	hierBarWidth = 8 // Positions in bar = depth levels

	// Block characters for intensity levels (sparse to dense)
	hierEmpty  = "░"
	hierLight  = "▒"
	hierMedium = "▓"
	hierFull   = "█"
)

// Depth-based color gradient (shallow to deep)
var depthColors = []string{
	"\033[32m",  // Green (depth 0)
	"\033[36m",  // Cyan (depth 1)
	"\033[34m",  // Blue (depth 2)
	"\033[35m",  // Magenta (depth 3)
	"\033[91m",  // Bright red (depth 4)
	"\033[93m",  // Bright yellow (depth 5)
	"\033[95m",  // Bright magenta (depth 6)
	"\033[97m",  // Bright white (depth 7+)
}

// HierDirStats holds stats with per-depth breakdown.
type HierDirStats struct {
	Name       string
	ByDepth    []int  // Changes at each depth level
	Total      int
	MaxDepth   int
	FileCount  int
	HasNew     bool
	DeepestDir string
}

// HierarchicalSparklineRenderer renders diff stats as depth-distribution bars.
// Format: dir ░░▒▓██░░ - each position = depth, intensity = changes
type HierarchicalSparklineRenderer struct {
	UseColor bool
	w        io.Writer
}

// NewHierarchicalSparklineRenderer creates a hierarchical sparkline renderer.
func NewHierarchicalSparklineRenderer(w io.Writer, useColor bool) *HierarchicalSparklineRenderer {
	return &HierarchicalSparklineRenderer{UseColor: useColor, w: w}
}

// Render outputs diff stats as depth-distribution sparklines.
func (r *HierarchicalSparklineRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Aggregate by top-level directory with depth breakdown
	dirs := aggregateByDirWithDepth(stats.Files)

	// Sort by total changes descending
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Total > dirs[j].Total
	})

	// Find max at any single depth for scaling
	maxAtDepth := 0
	for _, d := range dirs {
		for _, v := range d.ByDepth {
			if v > maxAtDepth {
				maxAtDepth = v
			}
		}
	}

	// Render each directory
	var parts []string
	for _, d := range dirs {
		parts = append(parts, r.formatDir(d, maxAtDepth))
	}

	fmt.Fprintln(r.w, strings.Join(parts, " "))
}

// formatDir formats a directory with depth-distribution bar.
func (r *HierarchicalSparklineRenderer) formatDir(d HierDirStats, maxAtDepth int) string {
	var sb strings.Builder

	// Show the deepest path as the label
	displayName := d.Name
	if d.DeepestDir != "" {
		displayName = d.DeepestDir
	}

	nameColor := ColorDir
	if d.HasNew {
		nameColor = ColorNew
	}

	sb.WriteString(r.color(nameColor))
	sb.WriteString(displayName)
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")

	// Build bar: each position = depth level
	for depth := 0; depth < hierBarWidth; depth++ {
		changes := 0
		if depth < len(d.ByDepth) {
			changes = d.ByDepth[depth]
		}

		// Select block character based on intensity
		block := r.intensityBlock(changes, maxAtDepth)

		// Color based on depth
		color := depthColors[depth%len(depthColors)]

		sb.WriteString(r.color(color))
		sb.WriteString(block)
		sb.WriteString(r.color(ColorReset))
	}

	return sb.String()
}

// intensityBlock returns block character based on change intensity.
func (r *HierarchicalSparklineRenderer) intensityBlock(changes, maxChanges int) string {
	if changes == 0 || maxChanges == 0 {
		return hierEmpty
	}

	ratio := float64(changes) / float64(maxChanges)
	switch {
	case ratio >= 0.75:
		return hierFull
	case ratio >= 0.5:
		return hierMedium
	case ratio >= 0.25:
		return hierLight
	default:
		return hierLight // At least light if any changes
	}
}

// color returns the ANSI code if color is enabled.
func (r *HierarchicalSparklineRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

// aggregateByDirWithDepth groups files by top-level directory,
// tracking change distribution across depth levels.
func aggregateByDirWithDepth(files []diff.FileStat) []HierDirStats {
	dirMap := make(map[string]*HierDirStats)

	for _, f := range files {
		topDir := getTopDir(f.Path)
		depth := strings.Count(f.Path, "/")

		if _, ok := dirMap[topDir]; !ok {
			dirMap[topDir] = &HierDirStats{
				Name:    topDir,
				ByDepth: make([]int, hierBarWidth),
			}
		}
		d := dirMap[topDir]

		changes := f.Additions + f.Deletions

		// Record changes at this depth level
		depthIndex := depth
		if depthIndex >= hierBarWidth {
			depthIndex = hierBarWidth - 1
		}
		d.ByDepth[depthIndex] += changes
		d.Total += changes
		d.FileCount++

		if depth > d.MaxDepth {
			d.MaxDepth = depth
			// Track deepest directory path
			lastSlash := strings.LastIndex(f.Path, "/")
			if lastSlash > 0 {
				d.DeepestDir = f.Path[:lastSlash]
			}
		}

		if f.IsUntracked {
			d.HasNew = true
		}
	}

	// Convert to slice
	result := make([]HierDirStats, 0, len(dirMap))
	for _, d := range dirMap {
		result = append(result, *d)
	}
	return result
}

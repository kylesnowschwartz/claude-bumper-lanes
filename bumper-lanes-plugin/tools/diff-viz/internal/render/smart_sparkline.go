// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

const smartBarWidth = 6 // Fixed width for sparkline bars

// SmartSparklineRenderer renders diff stats with depth-aware aggregation.
// Groups files at depth 2, shows file counts, preserves structure.
// Format: src/lib(2) ████ render(1) ██ main.go ░ │ tests(1) ██████
type SmartSparklineRenderer struct {
	UseColor bool
	w        io.Writer
}

// NewSmartSparklineRenderer creates a smart sparkline renderer.
func NewSmartSparklineRenderer(w io.Writer, useColor bool) *SmartSparklineRenderer {
	return &SmartSparklineRenderer{UseColor: useColor, w: w}
}

// Render outputs diff stats with depth-2 aggregation.
func (r *SmartSparklineRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Group by top-level directory, then by depth-2 path
	topDirs := GroupByTopDir(stats.Files)

	// Find max total for scaling
	maxTotal := 0
	for _, segments := range topDirs {
		for _, seg := range segments {
			if total := seg.Total(); total > maxTotal {
				maxTotal = total
			}
		}
	}

	// Sort top-level dirs by total changes
	sortedTops := SortTopDirs(topDirs)

	// Render each top-level directory
	var topParts []string
	for _, topDir := range sortedTops {
		segments := topDirs[topDir]
		topParts = append(topParts, r.formatTopDir(topDir, segments, maxTotal))
	}

	// Join top-level dirs with separator
	fmt.Fprintln(r.w, strings.Join(topParts, Separator(r.UseColor)))
}

// formatTopDir formats all segments within a top-level directory.
func (r *SmartSparklineRenderer) formatTopDir(topDir string, segments []PathSegment, maxTotal int) string {
	var parts []string

	for i, seg := range segments {
		var sb strings.Builder

		// For first segment, include top-level dir prefix
		if i == 0 && topDir != seg.SubPath {
			sb.WriteString(r.color(ColorDir))
			sb.WriteString(topDir)
			sb.WriteString("/")
			sb.WriteString(r.color(ColorReset))
		}

		// Segment name with appropriate color
		nameColor := ColorDir
		if seg.HasNew {
			nameColor = ColorNew
		}
		if seg.IsFile {
			nameColor = ColorReset
			if seg.HasNew {
				nameColor = ColorNew
			}
		}

		sb.WriteString(r.color(nameColor))
		sb.WriteString(seg.SubPath)
		sb.WriteString(r.color(ColorReset))

		// File count indicator for aggregated groups
		if !seg.IsFile && seg.FileCount > 1 {
			sb.WriteString(r.color(ColorFile))
			sb.WriteString(fmt.Sprintf("(%d)", seg.FileCount))
			sb.WriteString(r.color(ColorReset))
		}

		sb.WriteString(" ")

		// Sparkline bar
		sb.WriteString(r.formatBar(seg.Add, seg.Del))

		parts = append(parts, sb.String())
	}

	return strings.Join(parts, " ")
}

// formatBar creates a sparkline bar with ratio-split coloring.
func (r *SmartSparklineRenderer) formatBar(add, del int) string {
	total := add + del
	filled := min(filledFromTotal(total), smartBarWidth)
	block := blockChar(total)
	return RatioBar(add, del, filled, smartBarWidth, block, r.color)
}

// color returns the ANSI code if color is enabled.
func (r *SmartSparklineRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

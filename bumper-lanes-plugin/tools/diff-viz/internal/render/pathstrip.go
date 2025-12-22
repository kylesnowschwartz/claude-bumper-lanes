// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// stripBarWidth is the total width budget for bars in pathstrip mode.
// Each segment gets width proportional to its share of total changes.
const stripBarWidth = 40

// PathStripRenderer renders diff stats as proportional path segments.
// Format: src/lib:parser▓▓▓▓lex▓▓ render:tree▓▓▓ │ tests:unit▓▓▓▓
// Uses : for depth separator, bars inline after names.
type PathStripRenderer struct {
	UseColor bool
	w        io.Writer
}

// NewPathStripRenderer creates a path strip renderer.
func NewPathStripRenderer(w io.Writer, useColor bool) *PathStripRenderer {
	return &PathStripRenderer{UseColor: useColor, w: w}
}

// Render outputs diff stats as a single-line proportional strip.
func (r *PathStripRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Group by top-level directory, then by depth-2 path
	topGroups := GroupByTopDir(stats.Files)

	// Calculate total changes for proportional sizing
	grandTotal := stats.TotalAdd + stats.TotalDel
	if grandTotal == 0 {
		grandTotal = 1 // Avoid division by zero
	}

	// Sort top-level dirs by total changes descending
	sortedTops := SortTopDirs(topGroups)

	// Render each top-level directory
	var topParts []string
	for _, topDir := range sortedTops {
		segments := topGroups[topDir]
		topParts = append(topParts, r.formatTopDir(topDir, segments, grandTotal))
	}

	// Join with separator
	fmt.Fprintln(r.w, strings.Join(topParts, Separator(r.UseColor)))
}

// formatTopDir formats all segments within a top-level directory.
func (r *PathStripRenderer) formatTopDir(topDir string, segments []PathSegment, grandTotal int) string {
	var sb strings.Builder

	// Top-level dir prefix (skip for single root files where topDir == subPath)
	showPrefix := len(segments) > 1 || (len(segments) == 1 && topDir != segments[0].SubPath)
	if showPrefix && topDir != "" {
		sb.WriteString(r.color(ColorDir))
		sb.WriteString(topDir)
		sb.WriteString("/")
		sb.WriteString(r.color(ColorReset))
	}

	// Format each segment within this top-level dir
	for i, seg := range segments {
		if i > 0 {
			sb.WriteString(" ")
		}

		// Segment name with appropriate color
		nameColor := ColorReset
		if seg.HasNew {
			nameColor = ColorNew
		}
		sb.WriteString(r.color(nameColor))
		sb.WriteString(seg.SubPath)
		sb.WriteString(r.color(ColorReset))

		// Inline bar - proportional to this segment's share
		sb.WriteString(r.formatProportionalBar(seg.Add, seg.Del, grandTotal))
	}

	return sb.String()
}

// formatProportionalBar creates a bar sized proportionally to grandTotal.
func (r *PathStripRenderer) formatProportionalBar(add, del, grandTotal int) string {
	total := add + del
	if total == 0 {
		return BlockEmpty
	}

	// Calculate bar width as proportion of grand total
	filled := max(1, min((total*stripBarWidth)/grandTotal, stripBarWidth))
	block := blockChar(total)

	// RatioBar handles the add/del split and min 2 blocks logic
	return RatioBar(add, del, filled, filled, block, r.color)
}

// color returns the ANSI code if color is enabled.
func (r *PathStripRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

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
	sparkBarWidth = 6      // Fixed width for sparkline bars
	sparkFilled   = "█"    // U+2588 Full block
	sparkEmpty    = "░"    // U+2591 Light shade
)

// SparklineRenderer renders diff stats as compact horizontal bars.
// Format: src ████░░ tests ██░░░░ docs █░░░░░
type SparklineRenderer struct {
	UseColor bool
	w        io.Writer
}

// NewSparklineRenderer creates a sparkline renderer.
func NewSparklineRenderer(w io.Writer, useColor bool) *SparklineRenderer {
	return &SparklineRenderer{UseColor: useColor, w: w}
}

// Render outputs diff stats as sparkline bars.
func (r *SparklineRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Aggregate by top-level directory (reuse from collapsed)
	dirs := aggregateByDir(stats.Files)

	// Sort by total changes descending
	sort.Slice(dirs, func(i, j int) bool {
		return (dirs[i].Add + dirs[i].Del) > (dirs[j].Add + dirs[j].Del)
	})

	// Find max for scaling
	maxTotal := 0
	for _, d := range dirs {
		total := d.Add + d.Del
		if total > maxTotal {
			maxTotal = total
		}
	}

	// Render each directory
	var parts []string
	for _, d := range dirs {
		parts = append(parts, r.formatDir(d, maxTotal))
	}

	fmt.Fprintln(r.w, strings.Join(parts, " "))
}

// formatDir formats a single directory with sparkline bar.
func (r *SparklineRenderer) formatDir(d DirStats, maxTotal int) string {
	// Directory name with color
	nameColor := ColorDir
	if d.HasNew {
		nameColor = ColorNew
	}

	var sb strings.Builder
	sb.WriteString(r.color(nameColor))
	sb.WriteString(d.Name)
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")

	// Calculate filled blocks (scale to bar width)
	total := d.Add + d.Del
	filled := 0
	if maxTotal > 0 {
		filled = (total * sparkBarWidth) / maxTotal
		// Ensure at least 1 filled if there are changes
		if filled == 0 && total > 0 {
			filled = 1
		}
	}
	empty := sparkBarWidth - filled

	// Bar with color (green for additions, use additions color)
	sb.WriteString(r.color(ColorAdd))
	sb.WriteString(strings.Repeat(sparkFilled, filled))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(strings.Repeat(sparkEmpty, empty))

	return sb.String()
}

// color returns the ANSI code if color is enabled.
func (r *SparklineRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

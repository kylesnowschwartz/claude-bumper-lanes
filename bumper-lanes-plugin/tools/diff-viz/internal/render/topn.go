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
	topnBarWidth = 10    // Width of the sparkline bar
	topnDefault  = 5     // Default number of files to show
	topnFilled   = "█"   // U+2588 Full block
	topnMedium   = "▓"   // U+2593 Dark shade
	topnLight    = "▒"   // U+2592 Medium shade
	topnEmpty    = "░"   // U+2591 Light shade
)

// TopNRenderer shows the N files with the most changes.
type TopNRenderer struct {
	N        int
	UseColor bool
	w        io.Writer
}

// NewTopNRenderer creates a top-N summary renderer.
func NewTopNRenderer(w io.Writer, useColor bool, n int) *TopNRenderer {
	if n <= 0 {
		n = topnDefault
	}
	return &TopNRenderer{N: n, UseColor: useColor, w: w}
}

// Render outputs the top N files by total changes.
func (r *TopNRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Sort files by total changes (descending)
	files := make([]diff.FileStat, len(stats.Files))
	copy(files, stats.Files)
	sort.Slice(files, func(i, j int) bool {
		totalI := files[i].Additions + files[i].Deletions
		totalJ := files[j].Additions + files[j].Deletions
		return totalI > totalJ
	})

	// Take top N
	showCount := r.N
	if showCount > len(files) {
		showCount = len(files)
	}
	topFiles := files[:showCount]

	// Find max path length for alignment (cap at 40 chars)
	maxPathLen := 0
	for _, f := range topFiles {
		if len(f.Path) > maxPathLen {
			maxPathLen = len(f.Path)
		}
	}
	if maxPathLen > 40 {
		maxPathLen = 40
	}

	// Print each file
	for _, f := range topFiles {
		r.renderFile(f, maxPathLen)
	}

	// Summary line
	r.renderSummary(stats, showCount)
}

// renderFile outputs a single file line.
func (r *TopNRenderer) renderFile(f diff.FileStat, maxPathLen int) {
	var sb strings.Builder

	// Path (truncated if needed, left-aligned)
	path := f.Path
	if len(path) > maxPathLen {
		path = "..." + path[len(path)-maxPathLen+3:]
	}

	pathColor := ColorReset
	if f.IsUntracked {
		pathColor = ColorNew
	}
	sb.WriteString("  ")
	sb.WriteString(r.color(pathColor))
	sb.WriteString(fmt.Sprintf("%-*s", maxPathLen, path))
	sb.WriteString(r.color(ColorReset))

	// Stats: +X -Y (right-aligned in fixed width)
	statsStr := r.formatStats(f.Additions, f.Deletions)
	sb.WriteString("  ")
	sb.WriteString(statsStr)

	// Sparkline bar
	sb.WriteString("  ")
	sb.WriteString(r.formatBar(f.Additions, f.Deletions))

	fmt.Fprintln(r.w, sb.String())
}

// formatStats returns colored +X -Y string.
func (r *TopNRenderer) formatStats(add, del int) string {
	var sb strings.Builder

	// Fixed width: +XXX -XXX (14 chars total)
	if add > 0 {
		sb.WriteString(r.color(ColorAdd))
		sb.WriteString(fmt.Sprintf("+%-4d", add))
		sb.WriteString(r.color(ColorReset))
	} else {
		sb.WriteString("     ")
	}

	if del > 0 {
		sb.WriteString(r.color(ColorDel))
		sb.WriteString(fmt.Sprintf("-%-4d", del))
		sb.WriteString(r.color(ColorReset))
	} else {
		sb.WriteString("     ")
	}

	return sb.String()
}

// formatBar creates a sparkline bar with absolute scaling.
// Same thresholds as smart_sparkline for consistency.
func (r *TopNRenderer) formatBar(add, del int) string {
	total := add + del
	if total == 0 {
		return strings.Repeat(topnEmpty, topnBarWidth)
	}

	// Absolute thresholds (same as smart_sparkline, scaled to 10-width bar)
	var filled int
	switch {
	case total >= 400:
		filled = 10
	case total >= 300:
		filled = 9
	case total >= 200:
		filled = 8
	case total >= 150:
		filled = 7
	case total >= 100:
		filled = 6
	case total >= 75:
		filled = 5
	case total >= 50:
		filled = 4
	case total >= 30:
		filled = 3
	case total >= 15:
		filled = 2
	default:
		filled = 1
	}

	// Block character based on magnitude
	block := topnLight
	switch {
	case total >= 200:
		block = topnFilled
	case total >= 100:
		block = topnMedium
	default:
		block = topnLight
	}

	// Color: green for adds, red if deletions dominate
	blockColor := ColorAdd
	if del > add {
		blockColor = ColorDel
	}

	var sb strings.Builder
	sb.WriteString(r.color(blockColor))
	sb.WriteString(strings.Repeat(block, filled))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(strings.Repeat(topnEmpty, topnBarWidth-filled))

	return sb.String()
}

// renderSummary outputs the totals line.
func (r *TopNRenderer) renderSummary(stats *diff.DiffStats, shown int) {
	fmt.Fprintln(r.w)

	// Total line
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(r.color(ColorAdd))
	sb.WriteString(fmt.Sprintf("+%d", stats.TotalAdd))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")
	sb.WriteString(r.color(ColorDel))
	sb.WriteString(fmt.Sprintf("-%d", stats.TotalDel))
	sb.WriteString(r.color(ColorReset))

	// File count with omission note
	if stats.TotalFiles > shown {
		sb.WriteString(fmt.Sprintf(" (%d of %d files)", shown, stats.TotalFiles))
	} else {
		sb.WriteString(fmt.Sprintf(" (%d files)", stats.TotalFiles))
	}

	fmt.Fprintln(r.w, sb.String())
}

// color returns the ANSI code if color is enabled.
func (r *TopNRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

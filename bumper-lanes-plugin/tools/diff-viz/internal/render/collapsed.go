// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// DirStats holds aggregated stats for a directory.
type DirStats struct {
	Name      string
	Add       int
	Del       int
	FileCount int
	HasNew    bool // Contains untracked files
}

// CollapsedRenderer renders diff stats as a compact single-line summary.
// Format: src/ +95 (5) │ tests/ +25 (1) │ docs/ +12 (2)
type CollapsedRenderer struct {
	UseColor bool
	w        io.Writer
}

// NewCollapsedRenderer creates a collapsed renderer.
func NewCollapsedRenderer(w io.Writer, useColor bool) *CollapsedRenderer {
	return &CollapsedRenderer{UseColor: useColor, w: w}
}

// Render outputs diff stats as collapsed directory summaries.
func (r *CollapsedRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Aggregate by top-level directory
	dirs := aggregateByDir(stats.Files)

	// Sort by additions descending (biggest changes first)
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Add > dirs[j].Add
	})

	// Render each directory
	var parts []string
	for _, d := range dirs {
		parts = append(parts, r.formatDir(d))
	}

	// Join with separator
	sep := " │ "
	if !r.UseColor {
		sep = " | "
	}
	fmt.Fprintln(r.w, strings.Join(parts, sep))
}

// aggregateByDir groups file stats by top-level directory.
func aggregateByDir(files []diff.FileStat) []DirStats {
	dirMap := make(map[string]*DirStats)

	for _, f := range files {
		// Get top-level directory (or filename if at root)
		topDir := getTopDir(f.Path)

		if _, ok := dirMap[topDir]; !ok {
			dirMap[topDir] = &DirStats{Name: topDir}
		}
		d := dirMap[topDir]
		d.Add += f.Additions
		d.Del += f.Deletions
		d.FileCount++
		if f.IsUntracked {
			d.HasNew = true
		}
	}

	// Convert map to slice
	result := make([]DirStats, 0, len(dirMap))
	for _, d := range dirMap {
		result = append(result, *d)
	}
	return result
}

// getTopDir extracts the top-level directory from a path.
// Returns the filename itself if the file is at the root.
func getTopDir(path string) string {
	idx := strings.Index(path, "/")
	if idx == -1 {
		// File at root - use filename
		return path
	}
	return path[:idx]
}

// formatDir formats a single directory's stats.
func (r *CollapsedRenderer) formatDir(d DirStats) string {
	// Directory name - yellow if has new files, blue otherwise
	nameColor := ColorDir
	if d.HasNew {
		nameColor = ColorNew
	}

	var sb strings.Builder
	sb.WriteString(r.color(nameColor))
	sb.WriteString(d.Name)
	// Add trailing slash for directories (if it contains a subpath)
	if strings.Contains(d.Name, "/") || d.FileCount > 1 {
		sb.WriteString("/")
	}
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")

	// Stats
	if d.Add > 0 {
		sb.WriteString(r.color(ColorAdd))
		sb.WriteString(fmt.Sprintf("+%d", d.Add))
		sb.WriteString(r.color(ColorReset))
	}
	if d.Del > 0 {
		if d.Add > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(r.color(ColorDel))
		sb.WriteString(fmt.Sprintf("-%d", d.Del))
		sb.WriteString(r.color(ColorReset))
	}

	// File count
	sb.WriteString(fmt.Sprintf(" (%d)", d.FileCount))

	return sb.String()
}

// color returns the ANSI code if color is enabled.
func (r *CollapsedRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

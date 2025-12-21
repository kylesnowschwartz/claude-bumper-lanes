// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

const (
	smartBarWidth = 6    // Fixed width for sparkline bars
	smartFilled   = "█"  // U+2588 Full block
	smartMedium   = "▓"  // U+2593 Dark shade
	smartLight    = "▒"  // U+2592 Medium shade
	smartEmpty    = "░"  // U+2591 Light shade
)

// SmartGroup represents files aggregated at depth 2.
// e.g., src/lib contains parser.go, lexer.go
type SmartGroup struct {
	TopDir    string         // Top-level: src, tests, docs
	SubPath   string         // Depth-2 path: lib, render, or filename for root files
	Files     []diff.FileStat
	TotalAdd  int
	TotalDel  int
	FileCount int
	HasNew    bool
	IsFile    bool           // True if SubPath is a single file (not aggregated)
}

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
	topDirs := r.groupByTopDir(stats.Files)

	// Find max total for scaling
	maxTotal := 0
	for _, groups := range topDirs {
		for _, g := range groups {
			total := g.TotalAdd + g.TotalDel
			if total > maxTotal {
				maxTotal = total
			}
		}
	}

	// Sort top-level dirs by total changes
	sortedTops := r.sortTopDirs(topDirs)

	// Render each top-level directory
	var topParts []string
	for _, topDir := range sortedTops {
		groups := topDirs[topDir]
		topParts = append(topParts, r.formatTopDir(topDir, groups, maxTotal))
	}

	// Join top-level dirs with separator
	sep := " │ "
	if !r.UseColor {
		sep = " | "
	}
	fmt.Fprintln(r.w, strings.Join(topParts, sep))
}

// groupByTopDir groups files first by top-level dir, then by depth-2 path.
func (r *SmartSparklineRenderer) groupByTopDir(files []diff.FileStat) map[string][]SmartGroup {
	// First pass: group everything
	groupMap := make(map[string]map[string]*SmartGroup)

	for _, f := range files {
		topDir, subPath, isFile := r.getGroupPath(f.Path)

		if groupMap[topDir] == nil {
			groupMap[topDir] = make(map[string]*SmartGroup)
		}

		if groupMap[topDir][subPath] == nil {
			groupMap[topDir][subPath] = &SmartGroup{
				TopDir:  topDir,
				SubPath: subPath,
				IsFile:  isFile,
			}
		}

		g := groupMap[topDir][subPath]
		g.Files = append(g.Files, f)
		g.TotalAdd += f.Additions
		g.TotalDel += f.Deletions
		g.FileCount++
		if f.IsUntracked {
			g.HasNew = true
		}
	}

	// Convert to slices, sorted by total changes within each top dir
	result := make(map[string][]SmartGroup)
	for topDir, subGroups := range groupMap {
		groups := make([]SmartGroup, 0, len(subGroups))
		for _, g := range subGroups {
			// If group has only 1 file, convert to file display
			if g.FileCount == 1 {
				g.SubPath = path.Base(g.Files[0].Path)
				g.IsFile = true
			}
			groups = append(groups, *g)
		}
		// Sort by total changes descending
		sort.Slice(groups, func(i, j int) bool {
			return (groups[i].TotalAdd + groups[i].TotalDel) > (groups[j].TotalAdd + groups[j].TotalDel)
		})
		result[topDir] = groups
	}

	return result
}

// getGroupPath extracts top-level dir and depth-2 grouping path.
// Returns (topDir, subPath, isFile)
func (r *SmartSparklineRenderer) getGroupPath(filePath string) (string, string, bool) {
	parts := strings.Split(filePath, "/")

	switch len(parts) {
	case 1:
		// Root file: README.md
		return parts[0], parts[0], true
	case 2:
		// Depth 1 file: src/main.go
		return parts[0], parts[1], true
	default:
		// Depth 2+: src/lib/parser.go -> group under "lib"
		return parts[0], parts[1], false
	}
}

// sortTopDirs returns top-level dirs sorted by total changes.
func (r *SmartSparklineRenderer) sortTopDirs(topDirs map[string][]SmartGroup) []string {
	type topTotal struct {
		name  string
		total int
	}

	totals := make([]topTotal, 0, len(topDirs))
	for name, groups := range topDirs {
		total := 0
		for _, g := range groups {
			total += g.TotalAdd + g.TotalDel
		}
		totals = append(totals, topTotal{name, total})
	}

	sort.Slice(totals, func(i, j int) bool {
		return totals[i].total > totals[j].total
	})

	result := make([]string, len(totals))
	for i, t := range totals {
		result[i] = t.name
	}
	return result
}

// formatTopDir formats all groups within a top-level directory.
func (r *SmartSparklineRenderer) formatTopDir(topDir string, groups []SmartGroup, maxTotal int) string {
	var parts []string

	for i, g := range groups {
		var sb strings.Builder

		// For first group, include top-level dir prefix
		if i == 0 && topDir != g.SubPath {
			sb.WriteString(r.color(ColorDir))
			sb.WriteString(topDir)
			sb.WriteString("/")
			sb.WriteString(r.color(ColorReset))
		}

		// Group name
		nameColor := ColorDir
		if g.HasNew {
			nameColor = ColorNew
		}
		if g.IsFile {
			nameColor = ColorReset // Files get default color
			if g.HasNew {
				nameColor = ColorNew
			}
		}

		sb.WriteString(r.color(nameColor))
		sb.WriteString(g.SubPath)
		sb.WriteString(r.color(ColorReset))

		// File count indicator for aggregated groups
		if !g.IsFile && g.FileCount > 1 {
			sb.WriteString(r.color(ColorFile))
			sb.WriteString(fmt.Sprintf("(%d)", g.FileCount))
			sb.WriteString(r.color(ColorReset))
		}

		sb.WriteString(" ")

		// Sparkline bar
		sb.WriteString(r.formatBar(g.TotalAdd, g.TotalDel, maxTotal))

		parts = append(parts, sb.String())
	}

	return strings.Join(parts, " ")
}

// formatBar creates a sparkline bar with absolute scaling.
// Uses fixed thresholds so bar size is consistent across diffs:
//   1-25 lines: 1 block, 26-50: 2, 51-100: 3, 101-200: 4, 201-400: 5, 400+: 6
func (r *SmartSparklineRenderer) formatBar(add, del, _ int) string {
	total := add + del
	if total == 0 {
		return strings.Repeat(smartEmpty, smartBarWidth)
	}

	// Absolute thresholds - full bar = significant change (400+ lines)
	var filled int
	switch {
	case total >= 400:
		filled = 6
	case total >= 200:
		filled = 5
	case total >= 100:
		filled = 4
	case total >= 50:
		filled = 3
	case total >= 25:
		filled = 2
	default:
		filled = 1
	}

	// Block character based on absolute magnitude
	block := smartLight
	switch {
	case total >= 200:
		block = smartFilled
	case total >= 100:
		block = smartMedium
	default:
		block = smartLight
	}

	// Color: red if deletions dominate, green otherwise
	blockColor := ColorAdd
	if del > add {
		blockColor = ColorDel
	}

	var sb strings.Builder
	sb.WriteString(r.color(blockColor))
	sb.WriteString(strings.Repeat(block, filled))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(strings.Repeat(smartEmpty, smartBarWidth-filled))

	return sb.String()
}

// color returns the ANSI code if color is enabled.
func (r *SmartSparklineRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

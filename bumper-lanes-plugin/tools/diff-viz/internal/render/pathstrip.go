// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"sort"
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

// pathSegment represents an aggregated path segment with its changes.
type pathSegment struct {
	topDir   string // Top-level directory
	subPath  string // Depth-2 subpath or filename
	files    []string
	add      int
	del      int
	hasNew   bool
}

// Render outputs diff stats as a single-line proportional strip.
func (r *PathStripRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build segments grouped by top-level dir
	topGroups := r.buildSegments(stats.Files)

	// Calculate total changes for proportional sizing
	grandTotal := stats.TotalAdd + stats.TotalDel
	if grandTotal == 0 {
		grandTotal = 1 // Avoid division by zero
	}

	// Sort top-level dirs by total changes descending
	sortedTops := r.sortTopDirs(topGroups)

	// Render each top-level directory
	var topParts []string
	for _, topDir := range sortedTops {
		segments := topGroups[topDir]
		topParts = append(topParts, r.formatTopDir(topDir, segments, grandTotal))
	}

	// Join with separator
	sep := " │ "
	if !r.UseColor {
		sep = " | "
	}
	fmt.Fprintln(r.w, strings.Join(topParts, sep))
}

// buildSegments groups files by top-level dir and depth-2 path.
func (r *PathStripRenderer) buildSegments(files []diff.FileStat) map[string][]pathSegment {
	segmentMap := make(map[string]map[string]*pathSegment)

	for _, f := range files {
		topDir, subPath := r.getSegmentPath(f.Path)

		if segmentMap[topDir] == nil {
			segmentMap[topDir] = make(map[string]*pathSegment)
		}
		if segmentMap[topDir][subPath] == nil {
			segmentMap[topDir][subPath] = &pathSegment{
				topDir:  topDir,
				subPath: subPath,
			}
		}

		seg := segmentMap[topDir][subPath]
		seg.files = append(seg.files, f.Path)
		seg.add += f.Additions
		seg.del += f.Deletions
		if f.IsUntracked {
			seg.hasNew = true
		}
	}

	// Convert to sorted slices
	result := make(map[string][]pathSegment)
	for topDir, subMap := range segmentMap {
		segments := make([]pathSegment, 0, len(subMap))
		for _, seg := range subMap {
			segments = append(segments, *seg)
		}
		// Sort by total changes descending
		sort.Slice(segments, func(i, j int) bool {
			totalI := segments[i].add + segments[i].del
			totalJ := segments[j].add + segments[j].del
			return totalI > totalJ
		})
		result[topDir] = segments
	}

	return result
}

// getSegmentPath extracts top-level dir and depth-2 grouping.
// Returns (topDir, subPath) where subPath is the depth-2 component or filename.
func (r *PathStripRenderer) getSegmentPath(filePath string) (string, string) {
	parts := strings.Split(filePath, "/")

	switch len(parts) {
	case 1:
		// Root file: README.md -> ("", "README.md")
		return "", parts[0]
	case 2:
		// Depth 1: src/main.go -> ("src", "main.go")
		return parts[0], parts[1]
	default:
		// Depth 2+: src/lib/parser.go -> ("src", "lib")
		return parts[0], parts[1]
	}
}

// sortTopDirs returns top-level dirs sorted by total changes descending.
func (r *PathStripRenderer) sortTopDirs(topGroups map[string][]pathSegment) []string {
	type dirTotal struct {
		name  string
		total int
	}

	totals := make([]dirTotal, 0, len(topGroups))
	for name, segments := range topGroups {
		total := 0
		for _, seg := range segments {
			total += seg.add + seg.del
		}
		totals = append(totals, dirTotal{name, total})
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

// formatTopDir formats all segments within a top-level directory.
func (r *PathStripRenderer) formatTopDir(topDir string, segments []pathSegment, grandTotal int) string {
	var sb strings.Builder

	// Top-level dir prefix (if not root files)
	if topDir != "" {
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

		// Segment name
		nameColor := ColorReset
		if seg.hasNew {
			nameColor = ColorNew
		}
		sb.WriteString(r.color(nameColor))

		// Use : separator if this is nested under topDir
		if topDir != "" && seg.subPath != topDir {
			sb.WriteString(seg.subPath)
		} else if topDir == "" {
			sb.WriteString(seg.subPath)
		} else {
			sb.WriteString(seg.subPath)
		}
		sb.WriteString(r.color(ColorReset))

		// Inline bar - proportional to this segment's share
		sb.WriteString(r.formatProportionalBar(seg.add, seg.del, grandTotal))
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

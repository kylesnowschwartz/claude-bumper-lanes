// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// Box-drawing characters for icicle rendering.
// Based on go-pretty's BoxStyleLight / lipgloss normalBorder.
type BoxStyle struct {
	TopLeft      string // ┌
	TopRight     string // ┐
	BottomLeft   string // └
	BottomRight  string // ┘
	LeftSep      string // ├
	RightSep     string // ┤
	TopSep       string // ┬
	BottomSep    string // ┴
	Cross        string // ┼
	Horizontal   string // ─
	Vertical     string // │
}

// DefaultBoxStyle returns the standard light box style.
func DefaultBoxStyle() BoxStyle {
	return BoxStyle{
		TopLeft:    "┌",
		TopRight:   "┐",
		BottomLeft: "└",
		BottomRight: "┘",
		LeftSep:    "├",
		RightSep:   "┤",
		TopSep:     "┬",
		BottomSep:  "┴",
		Cross:      "┼",
		Horizontal: "─",
		Vertical:   "│",
	}
}

// ASCIIBoxStyle returns ASCII-safe box characters.
func ASCIIBoxStyle() BoxStyle {
	return BoxStyle{
		TopLeft:    "+",
		TopRight:   "+",
		BottomLeft: "+",
		BottomRight: "+",
		LeftSep:    "+",
		RightSep:   "+",
		TopSep:     "+",
		BottomSep:  "+",
		Cross:      "+",
		Horizontal: "-",
		Vertical:   "|",
	}
}

// IcicleCell represents a cell at a specific depth level.
type IcicleCell struct {
	Label    string // Display name (dir or file name)
	Path     string // Full path for this cell
	Total    int    // Total changes (add + del)
	Add      int    // Additions
	Del      int    // Deletions
	Start    int    // Pixel position of left edge (0-indexed)
	End      int    // Pixel position of right edge (exclusive)
	Children []int  // Indices into next level's cells that are children
}

// Width returns the cell width in characters.
func (c IcicleCell) Width() int {
	return c.End - c.Start
}

// Color returns the appropriate color code based on add/del ratio.
func (c IcicleCell) Color() string {
	switch {
	case c.Add > 0 && c.Del == 0:
		return ColorAdd
	case c.Del > 0 && c.Add == 0:
		return ColorDel
	default:
		return ColorDir
	}
}

// formatCentered returns the label centered within width, with ANSI color codes.
// The colorFn converts color codes to ANSI (or empty string if color disabled).
// reserveRight leaves space for a trailing separator (typically 1).
func (c IcicleCell) formatCentered(truncateFn func(string, int) string, colorFn func(string) string, width, reserveRight int) (content string, visualWidth int) {
	label := truncateFn(c.Label, width-reserveRight)
	labelLen := utf8.RuneCountInString(label)

	padding := width - labelLen - reserveRight
	if padding < 0 {
		padding = 0
	}
	leftPad := padding / 2
	rightPad := padding - leftPad

	var sb strings.Builder
	sb.WriteString(strings.Repeat(" ", leftPad))
	sb.WriteString(colorFn(c.Color()))
	sb.WriteString(label)
	sb.WriteString(colorFn(ColorReset))
	sb.WriteString(strings.Repeat(" ", rightPad))

	return sb.String(), leftPad + labelLen + rightPad
}

// IcicleRenderer renders diff stats as a horizontal icicle/flame chart.
// Width encodes magnitude, vertical stacking shows hierarchy.
type IcicleRenderer struct {
	UseColor     bool
	Width        int // Total width of the chart
	MaxDepth     int // Maximum depth levels to render (0 = unlimited)
	MinCellWidth int // Minimum width per cell (wider = less visual clutter)
	w            io.Writer
	style        BoxStyle
	levels       [][]IcicleCell // cells at each depth level
}

// NewIcicleRenderer creates an icicle renderer.
func NewIcicleRenderer(w io.Writer, useColor bool) *IcicleRenderer {
	style := DefaultBoxStyle()
	if !useColor {
		style = ASCIIBoxStyle()
	}
	return &IcicleRenderer{
		UseColor:     useColor,
		Width:        100, // Default width (standard terminal)
		MaxDepth:     4,   // Default max depth (shows 4 hierarchy levels)
		MinCellWidth: 12,   // Default min cell width
		w:            w,
		style:        style,
	}
}

// Render outputs the diff stats as a horizontal icicle chart.
func (r *IcicleRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build the hierarchical cell structure
	r.buildLevels(stats)

	if len(r.levels) == 0 || len(r.levels[0]) == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Render top border
	r.renderBorder(0, true)

	// Render each level with separators
	for depth := 0; depth < len(r.levels); depth++ {
		r.renderContentRow(depth)

		// Render separator (or bottom border if last)
		if depth < len(r.levels)-1 {
			r.renderSeparator(depth, depth+1)
		} else {
			r.renderBorder(depth, false)
		}
	}

	// Summary line
	fmt.Fprintf(r.w, "%s+%d%s %s-%d%s in %d files\n",
		r.color(ColorAdd), stats.TotalAdd, r.color(ColorReset),
		r.color(ColorDel), stats.TotalDel, r.color(ColorReset),
		stats.TotalFiles)
}

// buildLevels constructs the hierarchical cell structure from diff stats.
func (r *IcicleRenderer) buildLevels(stats *diff.DiffStats) {
	// Build tree first
	tree := r.buildTree(stats.Files)

	// Calculate total for proportional sizing
	totalChanges := stats.TotalAdd + stats.TotalDel
	if totalChanges == 0 {
		totalChanges = 1
	}

	// Build levels breadth-first
	r.levels = make([][]IcicleCell, 0)
	usableWidth := r.Width - 2 // Account for left/right borders

	// Level 0: root's children with proportional widths
	level0 := r.buildLevelCells(tree.Children, 0, usableWidth, totalChanges)
	if len(level0) == 0 {
		return
	}
	r.levels = append(r.levels, level0)

	// Build subsequent levels breadth-first
	for depth := 1; r.MaxDepth == 0 || depth < r.MaxDepth; depth++ {
		prevLevel := r.levels[depth-1]
		var nextLevel []IcicleCell

		for _, cell := range prevLevel {
			// Find the node for this cell
			node := FindNode(tree, cell.Path)
			if node == nil || !node.IsDir || len(node.Children) == 0 {
				continue
			}

			// Build children within this cell's bounds
			childCells := r.buildLevelCells(node.Children, cell.Start, cell.Width(), cell.Total)
			nextLevel = append(nextLevel, childCells...)
		}

		if len(nextLevel) == 0 {
			break // No more children to render
		}
		r.levels = append(r.levels, nextLevel)
	}
}


// buildTree constructs a tree from flat file paths.
// Uses shared tree utilities, then adds icicle-specific processing.
func (r *IcicleRenderer) buildTree(files []diff.FileStat) *TreeNode {
	root := BuildTreeFromFiles(files)

	// Calculate totals for directories (needed for proportional sizing)
	CalcTotals(root)

	// Collapse single-child chains (e.g., bumper-lanes-plugin/tools/diff-viz/ -> one node)
	CollapseSingleChildPaths(root)

	return root
}

// buildLevelCells creates cells for nodes within given bounds.
// Returns the cells without modifying r.levels.
func (r *IcicleRenderer) buildLevelCells(nodes []*TreeNode, startPos, availWidth, totalChanges int) []IcicleCell {
	if len(nodes) == 0 || availWidth < 1 {
		return nil
	}

	// Filter nodes with changes and sort by total descending
	sorted := make([]*TreeNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Add+n.Del > 0 {
			sorted = append(sorted, n)
		}
	}
	if len(sorted) == 0 {
		return nil
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Add+sorted[i].Del > sorted[j].Add+sorted[j].Del
	})

	// Calculate widths: reserve minimum for each, then distribute rest proportionally
	minReserved := len(sorted) * r.MinCellWidth
	if minReserved > availWidth {
		// Not enough space for all nodes - take what fits
		sorted = sorted[:availWidth/r.MinCellWidth]
		if len(sorted) == 0 {
			return nil
		}
		minReserved = len(sorted) * r.MinCellWidth
	}

	// Calculate proportional widths with minimum guarantee
	extraWidth := availWidth - minReserved
	widths := make([]int, len(sorted))
	for i, node := range sorted {
		nodeTotal := node.Add + node.Del
		extra := 0
		if extraWidth > 0 && totalChanges > 0 {
			extra = (nodeTotal * extraWidth) / totalChanges
		}
		widths[i] = r.MinCellWidth + extra
	}

	// Adjust to fill remaining space (avoid gaps)
	usedWidth := 0
	for _, w := range widths {
		usedWidth += w
	}
	if usedWidth < availWidth && len(widths) > 0 {
		widths[0] += availWidth - usedWidth // Give extra to largest
	}

	// Build cells
	cells := make([]IcicleCell, 0, len(sorted))
	pos := startPos

	for i, node := range sorted {
		width := widths[i]
		label := node.Name
		if node.IsDir {
			label += "/"
		}

		cells = append(cells, IcicleCell{
			Label: label,
			Path:  node.Path,
			Total: node.Add + node.Del,
			Add:   node.Add,
			Del:   node.Del,
			Start: pos,
			End:   pos + width,
		})

		pos += width
	}

	return cells
}

// renderBorder renders the top or bottom border.
func (r *IcicleRenderer) renderBorder(levelIdx int, isTop bool) {
	level := r.levels[levelIdx]
	boundaries := r.getBoundaries(levelIdx)

	var sb strings.Builder

	// Left corner
	if isTop {
		sb.WriteString(r.style.TopLeft)
	} else {
		sb.WriteString(r.style.BottomLeft)
	}

	// Horizontal line with separators at boundaries
	for pos := 1; pos < r.Width-1; pos++ {
		if boundaries[pos] {
			if isTop {
				sb.WriteString(r.style.TopSep)
			} else {
				sb.WriteString(r.style.BottomSep)
			}
		} else {
			sb.WriteString(r.style.Horizontal)
		}
	}

	// Right corner
	if isTop {
		sb.WriteString(r.style.TopRight)
	} else {
		sb.WriteString(r.style.BottomRight)
	}

	fmt.Fprintln(r.w, sb.String())
	_ = level // silence unused warning
}

// renderContentRow renders the content row for a level.
func (r *IcicleRenderer) renderContentRow(levelIdx int) {
	level := r.levels[levelIdx]

	// Get parent boundaries to draw separators in empty regions
	var parentBoundaries map[int]bool
	if levelIdx > 0 {
		parentBoundaries = r.getBoundaries(levelIdx - 1)
	}

	var sb strings.Builder
	sb.WriteString(r.style.Vertical)

	pos := 1 // Start after left border (position in visual columns)
	for i, cell := range level {
		// Fill gap before cell, respecting parent boundaries
		for pos < cell.Start+1 { // +1 for border offset
			if parentBoundaries[pos] {
				sb.WriteString(r.style.Vertical)
			} else {
				sb.WriteString(" ")
			}
			pos++
		}

		// Render centered, colored cell content
		content, visualWidth := cell.formatCentered(r.truncate, r.color, cell.Width(), 1)
		sb.WriteString(content)
		pos = cell.Start + 1 + visualWidth // +1 for left border offset

		// Cell separator (not after last cell)
		if i < len(level)-1 {
			sb.WriteString(r.style.Vertical)
			pos++
		}
	}

	// Fill remaining space, respecting parent boundaries
	for pos < r.Width-1 {
		if parentBoundaries[pos] {
			sb.WriteString(r.style.Vertical)
		} else {
			sb.WriteString(" ")
		}
		pos++
	}

	sb.WriteString(r.style.Vertical)
	fmt.Fprintln(r.w, sb.String())
}

// renderSeparator renders the separator row between two levels.
func (r *IcicleRenderer) renderSeparator(aboveIdx, belowIdx int) {
	aboveBoundaries := r.getBoundaries(aboveIdx)
	belowBoundaries := r.getBoundaries(belowIdx)

	var sb strings.Builder
	sb.WriteString(r.style.LeftSep)

	for pos := 1; pos < r.Width-1; pos++ {
		above := aboveBoundaries[pos]
		below := belowBoundaries[pos]

		switch {
		case above && below:
			sb.WriteString(r.style.Cross)
		case above:
			sb.WriteString(r.style.BottomSep)
		case below:
			sb.WriteString(r.style.TopSep)
		default:
			sb.WriteString(r.style.Horizontal)
		}
	}

	sb.WriteString(r.style.RightSep)
	fmt.Fprintln(r.w, sb.String())
}

// getBoundaries returns a map of pixel positions where vertical lines exist.
func (r *IcicleRenderer) getBoundaries(levelIdx int) map[int]bool {
	boundaries := make(map[int]bool)

	if levelIdx >= len(r.levels) {
		return boundaries
	}

	usableWidth := r.Width - 2 // Account for left/right borders
	for _, cell := range r.levels[levelIdx] {
		// Mark end position as boundary (between cells)
		// BUT don't mark the right edge - it's the box border, not an internal separator
		if cell.End < usableWidth {
			boundaries[cell.End] = true
		}
	}

	return boundaries
}

// truncate shortens a string to fit within maxLen runes, preserving trailing "/" for directories.
func (r *IcicleRenderer) truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runeCount := utf8.RuneCountInString(s)
	if runeCount <= maxLen {
		return s
	}

	// Preserve trailing "/" for directories
	isDir := len(s) > 0 && s[len(s)-1] == '/'
	if isDir {
		s = s[:len(s)-1] // Remove "/" temporarily
		maxLen--         // Reserve space for it
		runeCount--
	}

	var result string
	if maxLen <= 2 {
		// Too short for ellipsis, just truncate by runes
		result = string([]rune(s)[:min(runeCount, maxLen)])
	} else {
		// Truncate with ellipsis: "longname" -> "long~"
		result = string([]rune(s)[:maxLen-1]) + "~"
	}

	if isDir {
		result += "/"
	}
	return result
}

// color returns the ANSI code if color is enabled.
func (r *IcicleRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

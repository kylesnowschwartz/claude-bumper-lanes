// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"path/filepath"
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

// IcicleRenderer renders diff stats as a horizontal icicle/flame chart.
// Width encodes magnitude, vertical stacking shows hierarchy.
type IcicleRenderer struct {
	UseColor bool
	Width    int // Total width of the chart
	MaxDepth int // Maximum depth levels to render (0 = unlimited)
	w        io.Writer
	style    BoxStyle
	levels   [][]IcicleCell // cells at each depth level
}

// NewIcicleRenderer creates an icicle renderer.
func NewIcicleRenderer(w io.Writer, useColor bool) *IcicleRenderer {
	style := DefaultBoxStyle()
	if !useColor {
		style = ASCIIBoxStyle()
	}
	return &IcicleRenderer{
		UseColor: useColor,
		Width:    80, // Default width (standard terminal)
		MaxDepth: 3,  // Default max depth (shows 3 hierarchy levels)
		w:        w,
		style:    style,
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
			node := r.findNode(tree, cell.Path)
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

// findNode recursively finds a node by path in the tree.
func (r *IcicleRenderer) findNode(node *TreeNode, path string) *TreeNode {
	if node.Path == path {
		return node
	}
	for _, child := range node.Children {
		if found := r.findNode(child, path); found != nil {
			return found
		}
	}
	return nil
}

// buildTree constructs a tree from flat file paths (similar to TreeRenderer).
func (r *IcicleRenderer) buildTree(files []diff.FileStat) *TreeNode {
	root := &TreeNode{Name: "", IsDir: true}

	// Sort files for consistent output
	sortedFiles := make([]diff.FileStat, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Path < sortedFiles[j].Path
	})

	for _, f := range sortedFiles {
		r.insertPath(root, f)
	}

	// Calculate totals for directories
	r.calcTotals(root)

	// Collapse single-child chains (e.g., bumper-lanes-plugin/tools/diff-viz/ -> one node)
	r.collapseSingleChildPaths(root)

	return root
}

// collapseSingleChildPaths merges chains of single-child directories.
// e.g., a/b/c/d where each has one child becomes "a/b/c/d" as one node.
func (r *IcicleRenderer) collapseSingleChildPaths(node *TreeNode) {
	for i, child := range node.Children {
		// First, recursively collapse children
		r.collapseSingleChildPaths(child)

		// Then, if this child is a dir with exactly one child that's also a dir,
		// merge them together
		for child.IsDir && len(child.Children) == 1 && child.Children[0].IsDir {
			grandchild := child.Children[0]
			child.Name = child.Name + "/" + grandchild.Name
			child.Path = grandchild.Path
			child.Children = grandchild.Children
			// Note: Add/Del already calculated correctly since they propagate up
		}

		node.Children[i] = child
	}
}

// insertPath adds a file to the tree.
func (r *IcicleRenderer) insertPath(root *TreeNode, file diff.FileStat) {
	parts := strings.Split(file.Path, string(filepath.Separator))
	current := root

	for i, part := range parts {
		isFile := i == len(parts)-1

		var child *TreeNode
		for _, c := range current.Children {
			if c.Name == part {
				child = c
				break
			}
		}

		if child == nil {
			child = &TreeNode{
				Name:  part,
				Path:  strings.Join(parts[:i+1], string(filepath.Separator)),
				IsDir: !isFile,
			}
			current.Children = append(current.Children, child)
		}

		if isFile {
			child.Add = file.Additions
			child.Del = file.Deletions
		}

		current = child
	}
}

// calcTotals recursively calculates add/del totals for directories.
func (r *IcicleRenderer) calcTotals(node *TreeNode) (add, del int) {
	if !node.IsDir {
		return node.Add, node.Del
	}

	for _, child := range node.Children {
		childAdd, childDel := r.calcTotals(child)
		add += childAdd
		del += childDel
	}

	node.Add = add
	node.Del = del
	return add, del
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
	const minCellWidth = 8 // Minimum width per cell (wider = less visual clutter)
	minReserved := len(sorted) * minCellWidth
	if minReserved > availWidth {
		// Not enough space for all nodes - take what fits
		sorted = sorted[:availWidth/minCellWidth]
		if len(sorted) == 0 {
			return nil
		}
		minReserved = len(sorted) * minCellWidth
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
		widths[i] = minCellWidth + extra
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

	var sb strings.Builder
	sb.WriteString(r.style.Vertical)

	pos := 1 // Start after left border
	for i, cell := range level {
		// Fill gap if needed
		for pos < cell.Start+1 { // +1 for border offset
			sb.WriteString(" ")
			pos++
		}

		// Render cell content (centered label)
		cellWidth := cell.Width()
		label := r.truncate(cell.Label, cellWidth-1) // Leave room for separator

		// Color based on add/del ratio
		labelColor := ColorDir
		if cell.Add > 0 && cell.Del == 0 {
			labelColor = ColorAdd
		} else if cell.Del > 0 && cell.Add == 0 {
			labelColor = ColorDel
		}

		// Pad and center (use rune count for proper Unicode width)
		padding := cellWidth - utf8.RuneCountInString(label) - 1
		if padding < 0 {
			padding = 0
		}
		leftPad := padding / 2
		rightPad := padding - leftPad

		sb.WriteString(strings.Repeat(" ", max(0, leftPad)))
		sb.WriteString(r.color(labelColor))
		sb.WriteString(label)
		sb.WriteString(r.color(ColorReset))
		sb.WriteString(strings.Repeat(" ", max(0, rightPad)))

		// Track actual characters written
		charsWritten := max(0, leftPad) + utf8.RuneCountInString(label) + max(0, rightPad)
		pos = cell.Start + 1 + charsWritten // +1 for left border offset

		// Cell separator between cells (not after last cell)
		if i < len(level)-1 {
			sb.WriteString(r.style.Vertical)
			pos++
		}
	}

	// Fill remaining space
	for pos < r.Width-1 {
		sb.WriteString(" ")
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

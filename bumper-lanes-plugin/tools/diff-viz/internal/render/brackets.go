// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// BracketsRenderer renders diff stats as nested brackets showing hierarchy.
// Format: [src [lib parser██ lexer█] main█] [tests████]
//
// The hierarchy is encoded via bracket nesting, magnitude via bar width.
// Wraps intelligently at Width, keeping bracket groups intact.
type BracketsRenderer struct {
	UseColor   bool
	ShowCounts bool // Show +N-M instead of bars
	MaxBarLen  int  // Max bar characters per file (default 4)
	Width      int  // Max line width before wrapping (default 80)
	w          io.Writer
}

// NewBracketsRenderer creates a brackets renderer.
func NewBracketsRenderer(w io.Writer, useColor bool) *BracketsRenderer {
	return &BracketsRenderer{
		UseColor:   useColor,
		ShowCounts: true, // +N-M is more readable than bars in dense output
		MaxBarLen:  4,
		Width:      80,
		w:          w,
	}
}

// Render outputs diff stats as nested bracket notation.
func (r *BracketsRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build tree from files
	tree := buildBracketTree(stats.Files)

	// Find max value for scaling bars
	maxVal := r.findMaxValue(tree)

	// Render each top-level entry
	var parts []string
	for _, node := range tree {
		parts = append(parts, r.renderNode(node, maxVal, 0))
	}

	// Word-wrap style joining: keep bracket groups intact
	fmt.Fprintln(r.w, r.wrapJoin(parts))
}

// wrapJoin joins parts with word-wrap semantics.
// Each part stays intact; wraps to new line when width exceeded.
func (r *BracketsRenderer) wrapJoin(parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	var lines []string
	var currentLine strings.Builder
	currentWidth := 0

	for i, part := range parts {
		partWidth := visibleWidth(part)

		// First part on line, or fits on current line
		if currentWidth == 0 {
			currentLine.WriteString(part)
			currentWidth = partWidth
		} else if currentWidth+1+partWidth <= r.Width {
			// Fits with space separator
			currentLine.WriteString(" ")
			currentLine.WriteString(part)
			currentWidth += 1 + partWidth
		} else {
			// Wrap to new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(part)
			currentWidth = partWidth
		}

		// Handle last part
		if i == len(parts)-1 && currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		}
	}

	return strings.Join(lines, "\n")
}

// visibleWidth calculates display width excluding ANSI escape sequences.
func visibleWidth(s string) int {
	// Strip ANSI escape sequences: \033[...m
	inEscape := false
	width := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		width++
	}
	return width
}

// bracketNode represents a node in the bracket tree.
type bracketNode struct {
	Name     string
	Add      int
	Del      int
	IsDir    bool
	HasNew   bool
	Children []*bracketNode
}

func (n *bracketNode) Total() int {
	return n.Add + n.Del
}

// buildBracketTree constructs a tree from file stats.
// Groups files by path segments, aggregating stats at each level.
func buildBracketTree(files []diff.FileStat) []*bracketNode {
	root := &bracketNode{IsDir: true}

	for _, f := range files {
		parts := strings.Split(f.Path, "/")
		node := root

		for i, part := range parts {
			isLast := i == len(parts)-1

			// Find or create child
			var child *bracketNode
			for _, c := range node.Children {
				if c.Name == part {
					child = c
					break
				}
			}
			if child == nil {
				child = &bracketNode{
					Name:  part,
					IsDir: !isLast,
				}
				node.Children = append(node.Children, child)
			}

			// Accumulate stats at this level
			child.Add += f.Additions
			child.Del += f.Deletions
			if f.IsUntracked {
				child.HasNew = true
			}

			node = child
		}
	}

	// Sort children by total at each level (descending)
	sortBracketTree(root)

	return root.Children
}

// sortBracketTree recursively sorts children by total changes.
func sortBracketTree(node *bracketNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Total() > node.Children[j].Total()
	})
	for _, child := range node.Children {
		if child.IsDir {
			sortBracketTree(child)
		}
	}
}

// findMaxValue finds the maximum total across all leaf nodes.
func (r *BracketsRenderer) findMaxValue(nodes []*bracketNode) int {
	max := 0
	var walk func([]*bracketNode)
	walk = func(nodes []*bracketNode) {
		for _, n := range nodes {
			if !n.IsDir {
				if n.Total() > max {
					max = n.Total()
				}
			} else {
				walk(n.Children)
			}
		}
	}
	walk(nodes)
	return max
}

// Rainbow bracket colors - cycle through these based on depth
var bracketColors = []string{
	"\033[36m", // Cyan
	"\033[33m", // Yellow
	"\033[35m", // Magenta
	"\033[32m", // Green
	"\033[34m", // Blue
}

// renderNode recursively renders a node and its children.
func (r *BracketsRenderer) renderNode(node *bracketNode, maxVal int, depth int) string {
	var sb strings.Builder

	if node.IsDir {
		// Directory: [name children...] with rainbow brackets
		bracketColor := bracketColors[depth%len(bracketColors)]
		sb.WriteString(r.color(bracketColor))
		sb.WriteString("[")
		sb.WriteString(r.color(ColorReset))
		sb.WriteString(r.color(ColorDir))
		sb.WriteString(node.Name)
		sb.WriteString(r.color(ColorReset))

		// Render children at next depth
		for _, child := range node.Children {
			sb.WriteString(" ")
			sb.WriteString(r.renderNode(child, maxVal, depth+1))
		}
		sb.WriteString(r.color(bracketColor))
		sb.WriteString("]")
		sb.WriteString(r.color(ColorReset))
	} else {
		// File: name + bar or counts
		nameColor := ColorFile
		if node.HasNew {
			nameColor = ColorNew
		}
		sb.WriteString(r.color(nameColor))
		sb.WriteString(node.Name)
		sb.WriteString(r.color(ColorReset))

		if r.ShowCounts {
			// Show +N -M format with spacing
			if node.Add > 0 {
				sb.WriteString(" ")
				sb.WriteString(r.color(ColorAdd))
				sb.WriteString(fmt.Sprintf("+%d", node.Add))
				sb.WriteString(r.color(ColorReset))
			}
			if node.Del > 0 {
				sb.WriteString(" ")
				sb.WriteString(r.color(ColorDel))
				sb.WriteString(fmt.Sprintf("-%d", node.Del))
				sb.WriteString(r.color(ColorReset))
			}
		} else {
			// Show magnitude bar
			bar := r.makeBar(node.Total(), maxVal)
			if bar != "" {
				sb.WriteString(r.color(ColorAdd))
				sb.WriteString(bar)
				sb.WriteString(r.color(ColorReset))
			}
		}
	}

	return sb.String()
}

// makeBar creates a proportional bar based on value.
func (r *BracketsRenderer) makeBar(val, maxVal int) string {
	if maxVal == 0 || val == 0 {
		return ""
	}

	// Scale to MaxBarLen
	filled := (val * r.MaxBarLen) / maxVal
	if filled == 0 && val > 0 {
		filled = 1 // Always show at least one block for non-zero
	}

	return strings.Repeat("█", filled)
}

// color returns the ANSI code if color is enabled.
func (r *BracketsRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

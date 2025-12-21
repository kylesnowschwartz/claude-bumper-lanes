// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// ANSI color codes
const (
	ColorDir     = "\033[34m"     // Blue
	ColorFile    = "\033[38;5;8m" // Dark gray
	ColorNew     = "\033[33m"     // Yellow for untracked/new files
	ColorAdd     = "\033[32m"     // Green
	ColorDel     = "\033[31m"     // Red
	ColorReset   = "\033[0m"
)

// TreeNode represents a node in the file tree.
type TreeNode struct {
	Name        string
	Path        string
	IsDir       bool
	Add         int
	Del         int
	IsBinary    bool
	IsUntracked bool
	Children    []*TreeNode
}

// TreeRenderer renders diff stats as a hierarchical tree.
type TreeRenderer struct {
	UseColor bool
	w        io.Writer
}

// NewTreeRenderer creates a tree renderer.
func NewTreeRenderer(w io.Writer, useColor bool) *TreeRenderer {
	return &TreeRenderer{UseColor: useColor, w: w}
}

// Render outputs the diff stats as a tree.
func (r *TreeRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build tree from flat file list
	root := r.buildTree(stats.Files)

	// Render each top-level node
	for i, child := range root.Children {
		isLast := i == len(root.Children)-1
		r.renderNode(child, "", isLast, []bool{})
	}

	// Summary line
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "%s+%d%s %s-%d%s in %d files\n",
		r.color(ColorAdd), stats.TotalAdd, r.color(ColorReset),
		r.color(ColorDel), stats.TotalDel, r.color(ColorReset),
		stats.TotalFiles)
}

// buildTree constructs a tree from flat file paths.
func (r *TreeRenderer) buildTree(files []diff.FileStat) *TreeNode {
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

	return root
}

// insertPath adds a file to the tree, creating intermediate directories.
func (r *TreeRenderer) insertPath(root *TreeNode, file diff.FileStat) {
	parts := strings.Split(file.Path, string(filepath.Separator))
	current := root

	for i, part := range parts {
		isFile := i == len(parts)-1

		// Find or create child
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
			child.IsBinary = file.IsBinary
			child.IsUntracked = file.IsUntracked
		}

		current = child
	}
}

// renderNode outputs a single tree node with proper prefixes.
// parentIsLast tracks whether ancestors were last children (for prefix rendering).
func (r *TreeRenderer) renderNode(node *TreeNode, prefix string, isLast bool, parentIsLast []bool) {
	// Build prefix from parent state
	var sb strings.Builder
	for _, wasLast := range parentIsLast {
		if wasLast {
			sb.WriteString("    ")
		} else {
			sb.WriteString("│   ")
		}
	}

	// Add connector
	if isLast {
		sb.WriteString("└── ")
	} else {
		sb.WriteString("├── ")
	}

	// Render name with color
	if node.IsDir {
		fmt.Fprintf(r.w, "%s%s%s/%s\n", sb.String(), r.color(ColorDir), node.Name, r.color(ColorReset))
	} else {
		// File with stats - yellow for untracked, gray for tracked
		fileColor := ColorFile
		if node.IsUntracked {
			fileColor = ColorNew
		}
		stats := r.formatStats(node)
		fmt.Fprintf(r.w, "%s%s%s%s %s\n", sb.String(), r.color(fileColor), node.Name, r.color(ColorReset), stats)
	}

	// Render children
	newParentIsLast := append(parentIsLast, isLast)
	for i, child := range node.Children {
		childIsLast := i == len(node.Children)-1
		r.renderNode(child, prefix, childIsLast, newParentIsLast)
	}
}

// formatStats formats the +N -M stats for a file.
func (r *TreeRenderer) formatStats(node *TreeNode) string {
	if node.IsBinary {
		return "(binary)"
	}

	var parts []string
	if node.Add > 0 {
		parts = append(parts, fmt.Sprintf("%s+%d%s", r.color(ColorAdd), node.Add, r.color(ColorReset)))
	}
	if node.Del > 0 {
		parts = append(parts, fmt.Sprintf("%s-%d%s", r.color(ColorDel), node.Del, r.color(ColorReset)))
	}
	return strings.Join(parts, " ")
}

// color returns the ANSI code if color is enabled, empty string otherwise.
func (r *TreeRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

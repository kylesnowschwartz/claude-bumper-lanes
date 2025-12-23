// Package render provides diff visualization renderers.
package render

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// BuildTreeFromFiles constructs a tree from flat file paths.
// Files are sorted alphabetically for consistent output.
func BuildTreeFromFiles(files []diff.FileStat) *TreeNode {
	root := &TreeNode{Name: "", IsDir: true}

	// Sort files for consistent output
	sortedFiles := make([]diff.FileStat, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Path < sortedFiles[j].Path
	})

	for _, f := range sortedFiles {
		InsertPath(root, f)
	}

	return root
}

// InsertPath adds a file to the tree, creating intermediate directories.
func InsertPath(root *TreeNode, file diff.FileStat) {
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

// CalcTotals recursively calculates add/del totals for directories.
// Returns the total additions and deletions for the subtree.
func CalcTotals(node *TreeNode) (add, del int) {
	if !node.IsDir {
		return node.Add, node.Del
	}

	for _, child := range node.Children {
		childAdd, childDel := CalcTotals(child)
		add += childAdd
		del += childDel
	}

	node.Add = add
	node.Del = del
	return add, del
}

// CollapseSingleChildPaths merges chains of single-child directories.
// e.g., a/b/c/d where each has one child becomes "a/b/c/d" as one node.
func CollapseSingleChildPaths(node *TreeNode) {
	for i, child := range node.Children {
		// First, recursively collapse children
		CollapseSingleChildPaths(child)

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

// FindNode recursively finds a node by path in the tree.
// Returns nil if not found.
func FindNode(node *TreeNode, path string) *TreeNode {
	if node.Path == path {
		return node
	}
	for _, child := range node.Children {
		if found := FindNode(child, path); found != nil {
			return found
		}
	}
	return nil
}

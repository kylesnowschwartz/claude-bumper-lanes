// Package render provides diff visualization renderers.
package render

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

// PathSegment represents aggregated file changes at depth-2.
// Used by renderers that group files by directory structure.
type PathSegment struct {
	TopDir    string   // Top-level directory (e.g., "src", "tests")
	SubPath   string   // Depth-2 path or filename (e.g., "lib", "main.go")
	Files     []string // List of file paths in this segment
	Add       int      // Total additions
	Del       int      // Total deletions
	FileCount int      // Number of files
	HasNew    bool     // Contains untracked/new files
	IsFile    bool     // True if SubPath is a single file (not aggregated dir)
}

// Total returns the sum of additions and deletions.
func (p PathSegment) Total() int {
	return p.Add + p.Del
}

// Totaler is implemented by types that can report their total changes.
type Totaler interface {
	Total() int
}

// GetTopDir extracts the top-level directory from a file path.
// Returns the filename itself if the file is at the root.
func GetTopDir(path string) string {
	idx := strings.Index(path, "/")
	if idx == -1 {
		return path
	}
	return path[:idx]
}

// ParseDepth2Path extracts top-level dir and depth-2 grouping from a path.
// Returns (topDir, subPath, isFile).
//
// Examples:
//   - "README.md" -> ("README.md", "README.md", true)
//   - "src/main.go" -> ("src", "main.go", true)
//   - "src/lib/parser.go" -> ("src", "lib", false)
func ParseDepth2Path(filePath string) (topDir, subPath string, isFile bool) {
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

// GroupByTopDir groups files first by top-level dir, then by depth-2 path.
// Returns a map of topDir -> sorted slice of PathSegments.
// Single-file groups are converted to show the filename instead of dir name.
func GroupByTopDir(files []diff.FileStat) map[string][]PathSegment {
	// First pass: build nested map
	groupMap := make(map[string]map[string]*PathSegment)

	for _, f := range files {
		topDir, subPath, isFile := ParseDepth2Path(f.Path)

		if groupMap[topDir] == nil {
			groupMap[topDir] = make(map[string]*PathSegment)
		}

		if groupMap[topDir][subPath] == nil {
			groupMap[topDir][subPath] = &PathSegment{
				TopDir:  topDir,
				SubPath: subPath,
				IsFile:  isFile,
			}
		}

		seg := groupMap[topDir][subPath]
		seg.Files = append(seg.Files, f.Path)
		seg.Add += f.Additions
		seg.Del += f.Deletions
		seg.FileCount++
		if f.IsUntracked {
			seg.HasNew = true
		}
	}

	// Convert to slices, sorted by total changes within each top dir
	result := make(map[string][]PathSegment)
	for topDir, subGroups := range groupMap {
		segments := make([]PathSegment, 0, len(subGroups))
		for _, seg := range subGroups {
			// If group has only 1 file, convert to file display
			if seg.FileCount == 1 {
				seg.SubPath = filepath.Base(seg.Files[0])
				seg.IsFile = true
			}
			segments = append(segments, *seg)
		}
		// Sort by total changes descending
		sort.Slice(segments, func(i, j int) bool {
			return segments[i].Total() > segments[j].Total()
		})
		result[topDir] = segments
	}

	return result
}

// SortTopDirs returns top-level directory names sorted by total changes (descending).
// Works with any slice type that implements Totaler.
func SortTopDirs[T Totaler](groups map[string][]T) []string {
	type dirTotal struct {
		name  string
		total int
	}

	totals := make([]dirTotal, 0, len(groups))
	for name, items := range groups {
		total := 0
		for _, item := range items {
			total += item.Total()
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

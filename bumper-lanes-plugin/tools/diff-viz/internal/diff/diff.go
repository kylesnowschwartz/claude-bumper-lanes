// Package diff parses git diff output into structured data.
package diff

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// FileStat represents changes to a single file.
type FileStat struct {
	Path        string
	Additions   int
	Deletions   int
	IsBinary    bool
	IsUntracked bool
}

// FileStatJSON is the JSON-serializable representation of a file's stats.
type FileStatJSON struct {
	Path   string `json:"path"`
	Adds   int    `json:"adds"`
	Dels   int    `json:"dels"`
	Binary bool   `json:"binary,omitempty"`
	New    bool   `json:"new,omitempty"`
}

// TotalsJSON is the JSON-serializable representation of total stats.
type TotalsJSON struct {
	Adds      int `json:"adds"`
	Dels      int `json:"dels"`
	FileCount int `json:"fileCount"`
}

// StatsJSON is the JSON-serializable representation of diff stats.
// This is the output format for --stats-json flag.
type StatsJSON struct {
	Files  []FileStatJSON `json:"files"`
	Totals TotalsJSON     `json:"totals"`
}

// ToJSON converts DiffStats to JSON-serializable format.
func (s *DiffStats) ToJSON() StatsJSON {
	files := make([]FileStatJSON, len(s.Files))
	for i, f := range s.Files {
		files[i] = FileStatJSON{
			Path:   f.Path,
			Adds:   f.Additions,
			Dels:   f.Deletions,
			Binary: f.IsBinary,
			New:    f.IsUntracked,
		}
	}
	return StatsJSON{
		Files: files,
		Totals: TotalsJSON{
			Adds:      s.TotalAdd,
			Dels:      s.TotalDel,
			FileCount: s.TotalFiles,
		},
	}
}

// DiffStats holds all file changes from a git diff.
type DiffStats struct {
	Files      []FileStat
	TotalAdd   int
	TotalDel   int
	TotalFiles int
}

// GetDiffStats runs git diff --numstat and parses the output.
// args are passed directly to git diff (e.g., "HEAD", "--cached", "main..feature").
func GetDiffStats(args ...string) (*DiffStats, error) {
	cmdArgs := append([]string{"diff", "--numstat"}, args...)
	cmd := exec.Command("git", cmdArgs...)

	output, err := cmd.Output()
	if err != nil {
		// No changes or git error - return empty stats
		return &DiffStats{}, nil
	}

	return ParseNumstat(string(output))
}

// ParseNumstat parses git diff --numstat output.
// Format: "additions\tdeletions\tpath" or "-\t-\tpath" for binary files.
func ParseNumstat(output string) (*DiffStats, error) {
	stats := &DiffStats{}
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		file := FileStat{Path: parts[2]}

		if parts[0] == "-" {
			// Binary file
			file.IsBinary = true
		} else {
			file.Additions, _ = strconv.Atoi(parts[0])
			file.Deletions, _ = strconv.Atoi(parts[1])
		}

		stats.Files = append(stats.Files, file)
		stats.TotalAdd += file.Additions
		stats.TotalDel += file.Deletions
	}

	stats.TotalFiles = len(stats.Files)
	return stats, scanner.Err()
}

// GetUntrackedFiles returns stats for untracked files (additions only).
func GetUntrackedFiles() ([]FileStat, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil // No untracked files or git error
	}

	var files []FileStat
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		path := scanner.Text()
		if path == "" {
			continue
		}

		lines := countLines(path)
		file := FileStat{
			Path:        path,
			IsUntracked: true,
		}
		if lines == -1 {
			file.IsBinary = true
		} else {
			file.Additions = lines
		}
		files = append(files, file)
	}

	return files, scanner.Err()
}

// countLines counts lines in a file (for untracked files).
// Returns -1 for binary files.
func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	if len(data) == 0 {
		return 0
	}
	// Check for binary: look for null bytes in first 8KB
	checkLen := 8192
	if len(data) < checkLen {
		checkLen = len(data)
	}
	if bytes.Contains(data[:checkLen], []byte{0}) {
		return -1 // Binary file
	}
	// Count newlines, add 1 if file doesn't end with newline
	count := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		count++
	}
	return count
}

// GetAllStats returns diff stats including untracked files.
func GetAllStats(args ...string) (*DiffStats, error) {
	stats, err := GetDiffStats(args...)
	if err != nil {
		return nil, err
	}

	// Only include untracked for working tree diffs (no args or just "HEAD")
	includeUntracked := len(args) == 0 || (len(args) == 1 && args[0] == "HEAD")

	if includeUntracked {
		untracked, _ := GetUntrackedFiles()
		for _, f := range untracked {
			stats.Files = append(stats.Files, f)
			stats.TotalAdd += f.Additions
			stats.TotalFiles++
		}
	}

	return stats, nil
}

// GetTreeDiffStats compares two git tree SHAs using git diff-tree.
// This is used for comparing against a baseline snapshot.
func GetTreeDiffStats(baseTree, currentTree string) (*DiffStats, error) {
	// git diff-tree --numstat baseline current
	cmd := exec.Command("git", "diff-tree", "--numstat", "-r", baseTree, currentTree)
	output, err := cmd.Output()
	if err != nil {
		return &DiffStats{}, nil
	}

	stats, err := ParseNumstat(string(output))
	if err != nil {
		return nil, err
	}

	// Get file status (A=Added, M=Modified) for weighted scoring
	statusCmd := exec.Command("git", "diff-tree", "-r", "--name-status", "--diff-filter=AM", baseTree, currentTree)
	statusOutput, _ := statusCmd.Output()

	// Mark files as new or modified based on status
	statusLines := make(map[string]byte)
	scanner := bufio.NewScanner(bytes.NewReader(statusOutput))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) >= 2 && line[1] == '\t' {
			status := line[0]
			path := line[2:]
			statusLines[path] = status
		}
	}

	// Update file stats with new/modified status
	for i := range stats.Files {
		if status, ok := statusLines[stats.Files[i].Path]; ok {
			stats.Files[i].IsUntracked = (status == 'A') // Treat "Added" as new file
		}
	}

	return stats, nil
}

// CaptureCurrentTree returns the SHA of the current working tree.
// Uses a temporary index file to avoid modifying the real staging area.
// This matches the bash implementation in git-state.sh.
func CaptureCurrentTree() (string, error) {
	// Create temp index file
	tmpIndex, err := os.CreateTemp("", "git-index-*")
	if err != nil {
		return "", err
	}
	tmpIndexPath := tmpIndex.Name()
	tmpIndex.Close()
	defer os.Remove(tmpIndexPath)

	// Helper to run git commands with GIT_INDEX_FILE set
	gitWithTempIndex := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndexPath)
		return cmd
	}

	// Initialize temp index with HEAD tree (or empty if no commits)
	headRef, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil && len(headRef) > 0 {
		gitWithTempIndex("read-tree", strings.TrimSpace(string(headRef))).Run()
	} else {
		gitWithTempIndex("read-tree", "--empty").Run()
	}

	// Add tracked file changes (staged and unstaged)
	gitWithTempIndex("add", "-u", ".").Run()

	// Add untracked files (respecting .gitignore)
	lsCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedOutput, _ := lsCmd.Output()
	if len(untrackedOutput) > 0 {
		scanner := bufio.NewScanner(bytes.NewReader(untrackedOutput))
		for scanner.Scan() {
			path := scanner.Text()
			if path != "" {
				gitWithTempIndex("add", path).Run()
			}
		}
	}

	// Write tree from temp index
	writeCmd := gitWithTempIndex("write-tree")
	output, err := writeCmd.Output()
	if err != nil {
		return "", err
	}

	treeSHA := strings.TrimSpace(string(output))
	if treeSHA == "" {
		return "", exec.ErrNotFound
	}

	return treeSHA, nil
}

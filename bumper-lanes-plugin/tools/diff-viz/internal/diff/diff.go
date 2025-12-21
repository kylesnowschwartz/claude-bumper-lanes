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
	Path       string
	Additions  int
	Deletions  int
	IsBinary   bool
	IsUntracked bool
}

// DiffStats holds all file changes from a git diff.
type DiffStats struct {
	Files       []FileStat
	TotalAdd    int
	TotalDel    int
	TotalFiles  int
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

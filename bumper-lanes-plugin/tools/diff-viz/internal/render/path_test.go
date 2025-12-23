package render

import (
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

func TestGetTopDir(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"README.md", "README.md"},           // root file
		{"src/main.go", "src"},               // depth 1
		{"src/lib/parser.go", "src"},         // depth 2+
		{"a/b/c/d/e.go", "a"},                // deep nesting
		{".gitignore", ".gitignore"},         // dotfile at root
		{".github/workflows/ci.yml", ".github"}, // hidden dir
	}

	for _, tt := range tests {
		got := GetTopDir(tt.path)
		if got != tt.want {
			t.Errorf("GetTopDir(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestParseDepth2Path(t *testing.T) {
	tests := []struct {
		path    string
		topDir  string
		subPath string
		isFile  bool
	}{
		// Examples from docstring
		{"README.md", "README.md", "README.md", true},
		{"src/main.go", "src", "main.go", true},
		{"src/lib/parser.go", "src", "lib", false},

		// Additional cases
		{"go.mod", "go.mod", "go.mod", true},
		{"cmd/cli/main.go", "cmd", "cli", false},
		{"internal/render/bar.go", "internal", "render", false},
		{".github/workflows/test.yml", ".github", "workflows", false},
	}

	for _, tt := range tests {
		topDir, subPath, isFile := ParseDepth2Path(tt.path)
		if topDir != tt.topDir {
			t.Errorf("ParseDepth2Path(%q) topDir = %q, want %q", tt.path, topDir, tt.topDir)
		}
		if subPath != tt.subPath {
			t.Errorf("ParseDepth2Path(%q) subPath = %q, want %q", tt.path, subPath, tt.subPath)
		}
		if isFile != tt.isFile {
			t.Errorf("ParseDepth2Path(%q) isFile = %v, want %v", tt.path, isFile, tt.isFile)
		}
	}
}

func TestGroupByTopDir_SingleFile(t *testing.T) {
	files := []diff.FileStat{
		{Path: "src/main.go", Additions: 10, Deletions: 5},
	}

	result := GroupByTopDir(files)

	if len(result) != 1 {
		t.Fatalf("expected 1 top dir, got %d", len(result))
	}

	segments := result["src"]
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment in src, got %d", len(segments))
	}

	seg := segments[0]
	if seg.SubPath != "main.go" {
		t.Errorf("SubPath = %q, want %q", seg.SubPath, "main.go")
	}
	if !seg.IsFile {
		t.Error("IsFile = false, want true for single file")
	}
	if seg.Add != 10 || seg.Del != 5 {
		t.Errorf("Add/Del = %d/%d, want 10/5", seg.Add, seg.Del)
	}
}

func TestGroupByTopDir_AggregatesSubdirs(t *testing.T) {
	files := []diff.FileStat{
		{Path: "src/lib/a.go", Additions: 10},
		{Path: "src/lib/b.go", Additions: 20},
		{Path: "src/lib/c.go", Additions: 30},
	}

	result := GroupByTopDir(files)
	segments := result["src"]

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment (aggregated lib), got %d", len(segments))
	}

	seg := segments[0]
	if seg.SubPath != "lib" {
		t.Errorf("SubPath = %q, want %q (aggregated)", seg.SubPath, "lib")
	}
	if seg.IsFile {
		t.Error("IsFile = true, want false for aggregated dir")
	}
	if seg.Add != 60 {
		t.Errorf("Add = %d, want 60 (sum)", seg.Add)
	}
	if seg.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", seg.FileCount)
	}
}

func TestGroupByTopDir_SortsByTotal(t *testing.T) {
	files := []diff.FileStat{
		{Path: "src/small.go", Additions: 10},
		{Path: "src/large.go", Additions: 100},
		{Path: "src/medium.go", Additions: 50},
	}

	result := GroupByTopDir(files)
	segments := result["src"]

	// Should be sorted descending by total
	if segments[0].Add != 100 {
		t.Errorf("first segment Add = %d, want 100 (largest)", segments[0].Add)
	}
	if segments[1].Add != 50 {
		t.Errorf("second segment Add = %d, want 50 (medium)", segments[1].Add)
	}
	if segments[2].Add != 10 {
		t.Errorf("third segment Add = %d, want 10 (smallest)", segments[2].Add)
	}
}

func TestGroupByTopDir_TracksUntracked(t *testing.T) {
	files := []diff.FileStat{
		{Path: "src/new.go", Additions: 50, IsUntracked: true},
		{Path: "src/old.go", Additions: 50, IsUntracked: false},
	}

	result := GroupByTopDir(files)
	segments := result["src"]

	// Find the new file segment
	var newSeg *PathSegment
	for i := range segments {
		if segments[i].SubPath == "new.go" {
			newSeg = &segments[i]
			break
		}
	}

	if newSeg == nil {
		t.Fatal("new.go segment not found")
	}
	if !newSeg.HasNew {
		t.Error("HasNew = false, want true for untracked file")
	}
}

func TestGroupByTopDir_RootFiles(t *testing.T) {
	files := []diff.FileStat{
		{Path: "README.md", Additions: 10},
		{Path: "go.mod", Additions: 5},
	}

	result := GroupByTopDir(files)

	// Root files should each be their own top dir
	if _, ok := result["README.md"]; !ok {
		t.Error("README.md not found as top dir")
	}
	if _, ok := result["go.mod"]; !ok {
		t.Error("go.mod not found as top dir")
	}
}

// mockTotaler implements Totaler for testing SortTopDirs.
type mockTotaler int

func (m mockTotaler) Total() int { return int(m) }

func TestSortTopDirs(t *testing.T) {
	groups := map[string][]mockTotaler{
		"small":  {mockTotaler(10)},
		"large":  {mockTotaler(100)},
		"medium": {mockTotaler(50)},
	}

	result := SortTopDirs(groups)

	if len(result) != 3 {
		t.Fatalf("expected 3 dirs, got %d", len(result))
	}
	if result[0] != "large" {
		t.Errorf("result[0] = %q, want %q", result[0], "large")
	}
	if result[1] != "medium" {
		t.Errorf("result[1] = %q, want %q", result[1], "medium")
	}
	if result[2] != "small" {
		t.Errorf("result[2] = %q, want %q", result[2], "small")
	}
}

func TestSortTopDirs_SumsMultipleItems(t *testing.T) {
	groups := map[string][]mockTotaler{
		"many_small": {mockTotaler(10), mockTotaler(10), mockTotaler(10)}, // 30 total
		"one_large":  {mockTotaler(50)},                                    // 50 total
	}

	result := SortTopDirs(groups)

	if result[0] != "one_large" {
		t.Errorf("result[0] = %q, want %q (50 > 30)", result[0], "one_large")
	}
}


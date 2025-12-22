package diff

import (
	"testing"
)

func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFiles  int
		wantAdd    int
		wantDel    int
		wantBinary int // count of binary files
	}{
		{
			name:      "empty input",
			input:     "",
			wantFiles: 0,
		},
		{
			name:      "single file",
			input:     "10\t5\tsrc/main.go\n",
			wantFiles: 1,
			wantAdd:   10,
			wantDel:   5,
		},
		{
			name:      "multiple files",
			input:     "10\t5\tsrc/main.go\n20\t10\tsrc/util.go\n",
			wantFiles: 2,
			wantAdd:   30,
			wantDel:   15,
		},
		{
			name:       "binary file",
			input:      "-\t-\timage.png\n",
			wantFiles:  1,
			wantBinary: 1,
		},
		{
			name:       "mixed text and binary",
			input:      "50\t10\tcode.go\n-\t-\tlogo.png\n",
			wantFiles:  2,
			wantAdd:    50,
			wantDel:    10,
			wantBinary: 1,
		},
		{
			name:      "path with spaces",
			input:     "5\t0\tpath with spaces/file.go\n",
			wantFiles: 1,
			wantAdd:   5,
		},
		{
			name:      "path with tabs (edge case)",
			input:     "5\t0\tpath\twith\ttabs.go\n",
			wantFiles: 1,
			wantAdd:   5,
		},
		{
			name:      "no trailing newline",
			input:     "10\t5\tmain.go",
			wantFiles: 1,
			wantAdd:   10,
			wantDel:   5,
		},
		{
			name:      "blank lines ignored",
			input:     "10\t5\ta.go\n\n20\t10\tb.go\n",
			wantFiles: 2,
			wantAdd:   30,
			wantDel:   15,
		},
		{
			name:      "malformed line ignored",
			input:     "not\tvalid\n10\t5\tvalid.go\n",
			wantFiles: 1,
			wantAdd:   10,
			wantDel:   5,
		},
		{
			name:      "zero changes",
			input:     "0\t0\tempty.go\n",
			wantFiles: 1,
			wantAdd:   0,
			wantDel:   0,
		},
		{
			name:      "large numbers",
			input:     "10000\t5000\thuge.go\n",
			wantFiles: 1,
			wantAdd:   10000,
			wantDel:   5000,
		},
		{
			name:      "deep path",
			input:     "1\t0\ta/b/c/d/e/f/g/deeply/nested/file.go\n",
			wantFiles: 1,
			wantAdd:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNumstat(tt.input)
			if err != nil {
				t.Fatalf("ParseNumstat() error = %v", err)
			}

			if got.TotalFiles != tt.wantFiles {
				t.Errorf("TotalFiles = %d, want %d", got.TotalFiles, tt.wantFiles)
			}
			if got.TotalAdd != tt.wantAdd {
				t.Errorf("TotalAdd = %d, want %d", got.TotalAdd, tt.wantAdd)
			}
			if got.TotalDel != tt.wantDel {
				t.Errorf("TotalDel = %d, want %d", got.TotalDel, tt.wantDel)
			}

			// Count binary files
			binaryCount := 0
			for _, f := range got.Files {
				if f.IsBinary {
					binaryCount++
				}
			}
			if binaryCount != tt.wantBinary {
				t.Errorf("binary files = %d, want %d", binaryCount, tt.wantBinary)
			}
		})
	}
}

func TestParseNumstat_FilePaths(t *testing.T) {
	// Verify exact path parsing
	input := "10\t5\tsrc/main.go\n20\t10\tpkg/util/helper.go\n"
	got, err := ParseNumstat(input)
	if err != nil {
		t.Fatalf("ParseNumstat() error = %v", err)
	}

	if len(got.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got.Files))
	}

	// Check first file
	if got.Files[0].Path != "src/main.go" {
		t.Errorf("Files[0].Path = %q, want %q", got.Files[0].Path, "src/main.go")
	}
	if got.Files[0].Additions != 10 {
		t.Errorf("Files[0].Additions = %d, want 10", got.Files[0].Additions)
	}
	if got.Files[0].Deletions != 5 {
		t.Errorf("Files[0].Deletions = %d, want 5", got.Files[0].Deletions)
	}

	// Check second file
	if got.Files[1].Path != "pkg/util/helper.go" {
		t.Errorf("Files[1].Path = %q, want %q", got.Files[1].Path, "pkg/util/helper.go")
	}
}

func TestParseNumstat_BinaryFileDetails(t *testing.T) {
	input := "-\t-\timage.png\n"
	got, err := ParseNumstat(input)
	if err != nil {
		t.Fatalf("ParseNumstat() error = %v", err)
	}

	if len(got.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(got.Files))
	}

	f := got.Files[0]
	if f.Path != "image.png" {
		t.Errorf("Path = %q, want %q", f.Path, "image.png")
	}
	if !f.IsBinary {
		t.Error("IsBinary = false, want true")
	}
	if f.Additions != 0 {
		t.Errorf("Additions = %d, want 0 for binary", f.Additions)
	}
	if f.Deletions != 0 {
		t.Errorf("Deletions = %d, want 0 for binary", f.Deletions)
	}
}

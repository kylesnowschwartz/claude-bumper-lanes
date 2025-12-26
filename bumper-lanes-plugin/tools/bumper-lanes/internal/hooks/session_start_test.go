package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsOurWrapper(t *testing.T) {
	// Create temp dir for test files
	tmpDir := t.TempDir()

	// Create a file with our marker
	markerFile := filepath.Join(tmpDir, "with-marker.sh")
	if err := os.WriteFile(markerFile, []byte("#!/bin/bash\n"+wrapperMarker+" - DO NOT EDIT\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file without our marker
	noMarkerFile := filepath.Join(tmpDir, "no-marker.sh")
	if err := os.WriteFile(noMarkerFile, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file with our exact wrapper filename
	wrapperFile := filepath.Join(tmpDir, wrapperFileName)
	if err := os.WriteFile(wrapperFile, []byte("#!/bin/bash\necho wrapper"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"empty cmd", "", false},
		{"nonexistent file", "/nonexistent/path/script.sh", false},
		{"file with marker", markerFile, true},
		{"file without marker", noMarkerFile, false},
		{"wrapper filename match", wrapperFile, true},
		{"wrapper filename in different dir", filepath.Join("/some/other/path", wrapperFileName), true}, // filename match, file doesn't need to exist
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOurWrapper(tt.cmd, tmpDir)
			if got != tt.want {
				t.Errorf("isOurWrapper(%q, %q) = %v, want %v", tt.cmd, tmpDir, got, tt.want)
			}
		})
	}
}

func TestGenerateWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	wrapperPath := filepath.Join(tmpDir, "test-wrapper.sh")
	originalCmd := "/usr/bin/my-status-line"

	err := generateWrapper(wrapperPath, originalCmd, tmpDir)
	if err != nil {
		t.Fatalf("generateWrapper() error = %v", err)
	}

	// Read generated wrapper
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("failed to read wrapper: %v", err)
	}

	contentStr := string(content)

	// Check marker is present
	if !contains(contentStr, wrapperMarker) {
		t.Error("wrapper missing marker")
	}

	// Check original command is referenced
	if !contains(contentStr, originalCmd) {
		t.Error("wrapper missing original command")
	}

	// Check it's executable
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatalf("failed to stat wrapper: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Error("wrapper is not executable")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

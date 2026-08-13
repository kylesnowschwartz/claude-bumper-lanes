package hooks

import (
	"os"
	"testing"
)

// TestCaptureTree covers the tree-capture happy paths and the error path
// added when its git failures stopped being discarded.
func TestCaptureTree(t *testing.T) {
	t.Run("fresh repo with no tracked files still captures a tree", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		tree, err := CaptureTree()
		if err != nil {
			t.Fatalf("CaptureTree() error = %v, want nil", err)
		}
		if tree == "" {
			t.Error("CaptureTree() returned empty tree SHA")
		}
	})

	t.Run("untracked file is captured in the tree", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		if err := os.WriteFile(tmpDir+"/new.txt", []byte("hello"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		before, err := CaptureTree()
		if err != nil {
			t.Fatalf("CaptureTree() error = %v, want nil", err)
		}

		headTree := GetHeadTree()
		if before == headTree {
			t.Error("CaptureTree() did not include the untracked file")
		}
	})

	t.Run("error surfaces outside a git repository", func(t *testing.T) {
		tmpDir := t.TempDir() // not a git repo

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		if _, err := CaptureTree(); err == nil {
			t.Error("CaptureTree() error = nil, want non-nil outside a git repo")
		}
	})
}

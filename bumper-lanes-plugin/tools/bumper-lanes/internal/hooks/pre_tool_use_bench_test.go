package hooks

import (
	"os"
	"os/exec"
	"testing"
)

// BenchmarkPreToolUseCleanTreeCheck benchmarks the performance cost of the
// PreToolUse auto-reset check when StopTriggered=true.
//
// This measures the "hot path" that runs on every Write/Edit when threshold
// has been exceeded: CaptureTree + GetHeadTree + string comparison.
//
// Expected result: <10ms per operation on modern hardware.
// The PR claims "~60ms" but that seems too high for these operations.
func BenchmarkPreToolUseCleanTreeCheck(b *testing.B) {
	if !IsGitRepo() {
		b.Skip("Not in a git repo")
	}

	// Setup temp repo with commits
	tmpDir := b.TempDir()
	setupBenchGitRepo(b, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Create initial commit so we have HEAD
	os.WriteFile("file.txt", []byte("initial content\n"), 0644)
	exec.Command("git", "add", "file.txt").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	// Verify we have a clean tree (setup for benchmark)
	_, err := CaptureTree()
	if err != nil {
		b.Fatalf("Failed to capture baseline: %v", err)
	}

	if GetHeadTree() == "" {
		b.Fatalf("No HEAD tree")
	}

	// Benchmark the check operations (this is what PreToolUse does)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		currentTree, _ := CaptureTree()
		headTree := GetHeadTree()
		_ = currentTree == headTree
	}
}

// BenchmarkCaptureTreeOnly isolates just the CaptureTree operation.
func BenchmarkCaptureTreeOnly(b *testing.B) {
	if !IsGitRepo() {
		b.Skip("Not in a git repo")
	}

	tmpDir := b.TempDir()
	setupBenchGitRepo(b, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Create initial commit
	os.WriteFile("file.txt", []byte("initial content\n"), 0644)
	exec.Command("git", "add", "file.txt").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CaptureTree()
	}
}

// BenchmarkGetHeadTreeOnly isolates just the GetHeadTree operation.
func BenchmarkGetHeadTreeOnly(b *testing.B) {
	if !IsGitRepo() {
		b.Skip("Not in a git repo")
	}

	tmpDir := b.TempDir()
	setupBenchGitRepo(b, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Create initial commit
	os.WriteFile("file.txt", []byte("initial content\n"), 0644)
	exec.Command("git", "add", "file.txt").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetHeadTree()
	}
}

// setupBenchGitRepo initializes a git repo in tmpDir for benchmarking.
func setupBenchGitRepo(b *testing.B, tmpDir string) {
	b.Helper()

	// Use -b main to ensure consistent branch name
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		b.Fatalf("git init failed: %v", err)
	}

	// Configure git identity
	configCmds := [][]string{
		{"config", "user.name", "Benchmark"},
		{"config", "user.email", "benchmark@example.com"},
	}

	for _, args := range configCmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			b.Fatalf("git config failed: %v", err)
		}
	}
}

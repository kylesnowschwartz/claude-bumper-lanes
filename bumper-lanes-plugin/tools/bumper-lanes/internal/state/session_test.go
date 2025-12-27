package state

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionState_SaveLoad(t *testing.T) {
	// Create temp dir for test
	tmpDir := t.TempDir()
	checkpointDir := filepath.Join(tmpDir, "bumper-checkpoints")
	os.MkdirAll(checkpointDir, 0755)

	// Create state
	state := &SessionState{
		SessionID:      "test-session-123",
		BaselineTree:   "abc123def456",
		BaselineBranch: "main",
		Score:          100,
		CreatedAt:      "2025-01-01T00:00:00Z",
		ThresholdLimit: 400,
		RepoPath:       "/tmp/repo",
		StopTriggered:  false,
		Paused:         false,
	}

	// Write to temp location
	path := filepath.Join(checkpointDir, "session-"+state.SessionID)
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(path, data, 0644)

	// Read back
	readData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var loaded SessionState
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal state: %v", err)
	}

	if loaded.SessionID != state.SessionID {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, state.SessionID)
	}
	if loaded.BaselineTree != state.BaselineTree {
		t.Errorf("BaselineTree = %q, want %q", loaded.BaselineTree, state.BaselineTree)
	}
	if loaded.Score != state.Score {
		t.Errorf("Score = %d, want %d", loaded.Score, state.Score)
	}
}

func TestSessionState_ResetBaseline(t *testing.T) {
	state := &SessionState{
		SessionID:     "test-123",
		BaselineTree:  "old-tree",
		Score:         200,
		StopTriggered: true,
	}

	state.ResetBaseline("new-tree", "feature-branch")

	if state.BaselineTree != "new-tree" {
		t.Errorf("BaselineTree = %q, want %q", state.BaselineTree, "new-tree")
	}
	if state.Score != 0 {
		t.Errorf("Score = %d, want 0", state.Score)
	}
	if state.StopTriggered {
		t.Error("StopTriggered = true, want false")
	}
	if state.BaselineBranch != "feature-branch" {
		t.Errorf("BaselineBranch = %q, want %q", state.BaselineBranch, "feature-branch")
	}
}

func TestSessionState_SetScore(t *testing.T) {
	state := &SessionState{
		Score: 100,
	}

	state.SetScore(250)

	if state.Score != 250 {
		t.Errorf("Score = %d, want 250", state.Score)
	}
}

func TestCountCheckpoints(t *testing.T) {
	// Create temp dir and init as git repo
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Initialize git repo
	if err := os.WriteFile("test.txt", []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	runGit("add", ".")
	runGit("commit", "-m", "initial")

	// Get the checkpoint dir
	checkpointDir, err := GetCheckpointDir()
	if err != nil {
		t.Fatalf("Failed to get checkpoint dir: %v", err)
	}
	os.MkdirAll(checkpointDir, 0755)

	// Initially should be 0
	if count := CountCheckpoints(); count != 0 {
		t.Errorf("CountCheckpoints() = %d, want 0", count)
	}

	// Create some session files
	for i := 0; i < 5; i++ {
		path := filepath.Join(checkpointDir, "session-test-"+string(rune('a'+i)))
		os.WriteFile(path, []byte("{}"), 0644)
	}

	if count := CountCheckpoints(); count != 5 {
		t.Errorf("CountCheckpoints() = %d, want 5", count)
	}

	// .tmp files should not be counted
	os.WriteFile(filepath.Join(checkpointDir, "session-temp.tmp"), []byte("{}"), 0644)
	if count := CountCheckpoints(); count != 5 {
		t.Errorf("CountCheckpoints() with .tmp = %d, want 5", count)
	}

	// Non-session files should not be counted
	os.WriteFile(filepath.Join(checkpointDir, "other-file"), []byte("{}"), 0644)
	if count := CountCheckpoints(); count != 5 {
		t.Errorf("CountCheckpoints() with other file = %d, want 5", count)
	}
}

func TestCheckpointCountWarning(t *testing.T) {
	// Create temp dir and init as git repo
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Initialize git repo
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	os.WriteFile("test.txt", []byte("test"), 0644)
	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	runGit("add", ".")
	runGit("commit", "-m", "initial")

	checkpointDir, _ := GetCheckpointDir()
	os.MkdirAll(checkpointDir, 0755)

	// Under threshold - no warning
	for i := 0; i < 50; i++ {
		path := filepath.Join(checkpointDir, fmt.Sprintf("session-%03d", i))
		os.WriteFile(path, []byte("{}"), 0644)
	}
	if warning := CheckpointCountWarning(); warning != "" {
		t.Errorf("CheckpointCountWarning() at 50 = %q, want empty", warning)
	}

	// At threshold - should warn
	for i := 50; i < 100; i++ {
		path := filepath.Join(checkpointDir, fmt.Sprintf("session-%03d", i))
		os.WriteFile(path, []byte("{}"), 0644)
	}
	warning := CheckpointCountWarning()
	if warning == "" {
		t.Error("CheckpointCountWarning() at 100 = empty, want warning")
	}
	if !strings.Contains(warning, "100") {
		t.Errorf("Warning should contain count: %q", warning)
	}
	if !strings.Contains(warning, "rm -rf") {
		t.Errorf("Warning should contain cleanup command: %q", warning)
	}
}

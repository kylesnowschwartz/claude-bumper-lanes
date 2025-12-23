package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionState_SaveLoad(t *testing.T) {
	// Create temp dir for test
	tmpDir := t.TempDir()
	checkpointDir := filepath.Join(tmpDir, "bumper-checkpoints")
	os.MkdirAll(checkpointDir, 0755)

	// Create state
	state := &SessionState{
		SessionID:        "test-session-123",
		BaselineTree:     "abc123def456",
		BaselineBranch:   "main",
		PreviousTree:     "abc123def456",
		AccumulatedScore: 100,
		CreatedAt:        "2025-01-01T00:00:00Z",
		ThresholdLimit:   400,
		RepoPath:         "/tmp/repo",
		StopTriggered:    false,
		Paused:           false,
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
	if loaded.AccumulatedScore != state.AccumulatedScore {
		t.Errorf("AccumulatedScore = %d, want %d", loaded.AccumulatedScore, state.AccumulatedScore)
	}
}

func TestSessionState_ResetBaseline(t *testing.T) {
	state := &SessionState{
		SessionID:        "test-123",
		BaselineTree:     "old-tree",
		PreviousTree:     "old-tree",
		AccumulatedScore: 200,
		StopTriggered:    true,
	}

	state.ResetBaseline("new-tree", "feature-branch")

	if state.BaselineTree != "new-tree" {
		t.Errorf("BaselineTree = %q, want %q", state.BaselineTree, "new-tree")
	}
	if state.PreviousTree != "new-tree" {
		t.Errorf("PreviousTree = %q, want %q", state.PreviousTree, "new-tree")
	}
	if state.AccumulatedScore != 0 {
		t.Errorf("AccumulatedScore = %d, want 0", state.AccumulatedScore)
	}
	if state.StopTriggered {
		t.Error("StopTriggered = true, want false")
	}
	if state.BaselineBranch != "feature-branch" {
		t.Errorf("BaselineBranch = %q, want %q", state.BaselineBranch, "feature-branch")
	}
}

func TestSessionState_UpdateIncremental(t *testing.T) {
	state := &SessionState{
		PreviousTree:     "old-tree",
		AccumulatedScore: 100,
	}

	state.UpdateIncremental("new-tree", 250)

	if state.PreviousTree != "new-tree" {
		t.Errorf("PreviousTree = %q, want %q", state.PreviousTree, "new-tree")
	}
	if state.AccumulatedScore != 250 {
		t.Errorf("AccumulatedScore = %d, want 250", state.AccumulatedScore)
	}
}


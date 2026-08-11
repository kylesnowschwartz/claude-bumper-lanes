package events

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	return dir
}

func TestAppendWritesParseableJSONL(t *testing.T) {
	dir := setupGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	entries := []Entry{
		{SessionID: "s1", Event: SessionStart, Score: 0, Limit: 600, Cause: "startup"},
		{SessionID: "s1", Event: Trip, Score: 650, Limit: 600},
		{SessionID: "s1", Event: Reset, Score: 650, Limit: 600, Cause: CauseManual},
	}
	for _, e := range entries {
		if err := Append(e); err != nil {
			t.Fatalf("Append(%v) error: %v", e.Event, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, ".git", "bumper-checkpoints", "events.jsonl"))
	if err != nil {
		t.Fatalf("reading events.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != len(entries) {
		t.Fatalf("got %d lines, want %d", len(lines), len(entries))
	}
	for i, line := range lines {
		var got Entry
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}
		if got.TS == "" {
			t.Errorf("line %d missing ts", i)
		}
		if got.Event != entries[i].Event || got.Score != entries[i].Score || got.Cause != entries[i].Cause {
			t.Errorf("line %d = %+v, want event/score/cause of %+v", i, got, entries[i])
		}
	}
}

func TestAppendOutsideGitRepoIsNoop(t *testing.T) {
	dir := t.TempDir() // not a git repo
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	if err := Append(Entry{SessionID: "s1", Event: Pause}); err != nil {
		t.Errorf("Append outside git repo = %v, want nil (fail open)", err)
	}
}

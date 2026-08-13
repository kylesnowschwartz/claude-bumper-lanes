package state

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupCheckpointRepo creates a git repo (GetCheckpointDir shells out to
// git) with a checkpoint dir, chdirs into it, and returns the checkpoint
// path.
func setupCheckpointRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	if err := exec.Command("git", "init", "-q", tmpDir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(tmpDir)

	checkpointDir := filepath.Join(tmpDir, ".git", "bumper-checkpoints")
	os.MkdirAll(checkpointDir, 0755)
	return checkpointDir
}

// writeSessionFile persists a minimal state file with a controlled mtime.
func writeSessionFile(t *testing.T, dir, id string, age time.Duration) {
	t.Helper()
	s := &SessionState{SessionID: id, BaselineTree: "tree", ThresholdLimit: 600}
	data, _ := json.Marshal(s)
	path := filepath.Join(dir, "session-"+id)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", id, err)
	}
	mtime := time.Now().Add(-age)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", id, err)
	}
}

func TestLoadLatest(t *testing.T) {
	t.Run("single session loads", func(t *testing.T) {
		dir := setupCheckpointRepo(t)
		writeSessionFile(t, dir, "only", 5*time.Minute)
		sess, err := LoadLatest()
		if err != nil {
			t.Fatalf("LoadLatest: %v", err)
		}
		if sess.SessionID != "only" {
			t.Errorf("loaded %q, want only", sess.SessionID)
		}
	})

	t.Run("two active sessions refuse the fallback", func(t *testing.T) {
		dir := setupCheckpointRepo(t)
		writeSessionFile(t, dir, "alpha", 5*time.Minute)
		writeSessionFile(t, dir, "beta", 10*time.Minute)
		_, err := LoadLatest()
		if err == nil {
			t.Fatal("expected ambiguity refusal, got a session")
		}
		if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
			t.Errorf("error should name both candidates, got: %v", err)
		}
	})

	t.Run("a stale session does not block the active one", func(t *testing.T) {
		dir := setupCheckpointRepo(t)
		writeSessionFile(t, dir, "fresh", 5*time.Minute)
		writeSessionFile(t, dir, "yesterday", 25*time.Hour)
		sess, err := LoadLatest()
		if err != nil {
			t.Fatalf("LoadLatest: %v", err)
		}
		if sess.SessionID != "fresh" {
			t.Errorf("loaded %q, want fresh", sess.SessionID)
		}
	})

	t.Run("all stale falls back to the newest", func(t *testing.T) {
		dir := setupCheckpointRepo(t)
		writeSessionFile(t, dir, "older", 30*time.Hour)
		writeSessionFile(t, dir, "newer", 25*time.Hour)
		sess, err := LoadLatest()
		if err != nil {
			t.Fatalf("LoadLatest: %v", err)
		}
		if sess.SessionID != "newer" {
			t.Errorf("loaded %q, want newer", sess.SessionID)
		}
	})

	t.Run("empty checkpoint dir reports no session", func(t *testing.T) {
		setupCheckpointRepo(t)
		_, err := LoadLatest()
		if !errors.Is(err, ErrNoSession) {
			t.Errorf("want ErrNoSession, got: %v", err)
		}
	})
}

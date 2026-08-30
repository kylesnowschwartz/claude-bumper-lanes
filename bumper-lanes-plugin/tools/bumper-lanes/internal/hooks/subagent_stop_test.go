package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// dirtyTree writes a file large enough to exceed a low threshold.
func dirtyTree(t *testing.T, dir string) {
	t.Helper()
	var content strings.Builder
	content.WriteString("package main\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&content, "// line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "big-change.go"), []byte(content.String()), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
}

func TestSubagentStopTripsWhenOverThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	baseline, _ := git.CaptureTree()
	sessionID := "test-subagent-trip"
	sess, err := state.New(sessionID, baseline, "main", 30)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	sess.Save()

	dirtyTree(t, tmpDir)

	input := &hookio.Input{
		SessionID:      sessionID,
		HookEventName:  "SubagentStop",
		StopHookActive: false,
	}

	output := captureStdout(t, func() {
		if err := SubagentStop(input); err != nil {
			t.Errorf("SubagentStop() error = %v", err)
		}
	})

	var resp hookio.StopResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v\nOutput: %s", err, output)
	}
	if resp.Decision != "block" {
		t.Errorf("Decision = %q, want 'block'", resp.Decision)
	}
	if !strings.Contains(resp.Reason, "Wrap up now") {
		t.Errorf("Reason should instruct wrap-up, got: %s", resp.Reason)
	}
	if resp.TerminalSequence == "" {
		t.Error("fresh subagent trip should carry the desktop notification")
	}

	reloaded, _ := state.Load(sessionID)
	if !reloaded.StopTriggered {
		t.Error("StopTriggered should be true after subagent-stop trip")
	}
	if reloaded.Policy == nil {
		t.Error("Policy should be stamped at trip for Bash-invoked review-clear")
	}

	// The trip event carries the subagent-stop cause for the instrument.
	gitDirOut, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
	gitDir := strings.TrimSpace(string(gitDirOut))
	data, err := os.ReadFile(filepath.Join(gitDir, "bumper-checkpoints", "events.jsonl"))
	if err != nil {
		t.Fatalf("reading events.jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"event":"trip"`) || !strings.Contains(string(data), `"cause":"subagent-stop"`) {
		t.Errorf("events.jsonl missing trip with cause subagent-stop: %s", string(data))
	}
}

func TestSubagentStopSilentWhenAlreadyTripped(t *testing.T) {
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	baseline, _ := git.CaptureTree()
	sessionID := "test-subagent-already-tripped"
	sess, err := state.New(sessionID, baseline, "main", 30)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	sess.StopTriggered = true
	sess.Save()

	dirtyTree(t, tmpDir)

	// A reviewer/explorer subagent finishing while the session is already
	// tripped must stop undisturbed - it has nothing to summarize.
	input := &hookio.Input{
		SessionID:      sessionID,
		HookEventName:  "SubagentStop",
		StopHookActive: false,
	}

	output := captureStdout(t, func() {
		if err := SubagentStop(input); err != nil {
			t.Errorf("SubagentStop() error = %v", err)
		}
	})
	if output != "" {
		t.Errorf("already-tripped subagent stop should be silent, got: %s", output)
	}
}

func TestSubagentStopClearsBreakerOnRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	baseline, _ := git.CaptureTree()
	sessionID := "test-subagent-recovery"
	sess, err := state.New(sessionID, baseline, "main", 100)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	sess.StopTriggered = true
	sess.Score = 150
	sess.Save()

	// Tree matches baseline: the tripped score has been reverted.
	input := &hookio.Input{
		SessionID:      sessionID,
		HookEventName:  "SubagentStop",
		StopHookActive: false,
	}

	output := captureStdout(t, func() {
		if err := SubagentStop(input); err != nil {
			t.Errorf("SubagentStop() error = %v", err)
		}
	})
	if output != "" {
		t.Errorf("recovery at subagent stop should be silent, got: %s", output)
	}

	reloaded, _ := state.Load(sessionID)
	if reloaded.StopTriggered {
		t.Error("StopTriggered should clear when score recovers at a subagent stop")
	}
	if reloaded.Score != 0 {
		t.Errorf("Score = %d, want 0 (clean tree)", reloaded.Score)
	}
}

func TestSubagentStopAllowsUnderThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	baseline, _ := git.CaptureTree()
	sessionID := "test-subagent-under"
	sess, err := state.New(sessionID, baseline, "main", 1000)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	sess.Save()

	dirtyTree(t, tmpDir)

	input := &hookio.Input{
		SessionID:      sessionID,
		HookEventName:  "SubagentStop",
		StopHookActive: false,
	}

	output := captureStdout(t, func() {
		if err := SubagentStop(input); err != nil {
			t.Errorf("SubagentStop() error = %v", err)
		}
	})
	if output != "" {
		t.Errorf("under threshold should be silent, got: %s", output)
	}

	reloaded, _ := state.Load(sessionID)
	if reloaded.StopTriggered {
		t.Error("StopTriggered should stay false under threshold")
	}
	if reloaded.Score == 0 {
		t.Error("Score should update from the dirty tree at subagent stop")
	}
}

func TestSubagentStopSkipsWhenStopHookActive(t *testing.T) {
	input := &hookio.Input{
		SessionID:      "any",
		HookEventName:  "SubagentStop",
		StopHookActive: true,
	}
	if err := SubagentStop(input); err != nil {
		t.Errorf("SubagentStop() with StopHookActive=true should return nil, got: %v", err)
	}
}

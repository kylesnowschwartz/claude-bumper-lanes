package hooks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

func TestStopCumulativeStats(t *testing.T) {
	// Skip if not in a git repo or binary not built
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("uses BaselineTree not PreviousTree for breakdown", func(t *testing.T) {
		// Create a temp git repo
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Set up CLAUDE_PLUGIN_ROOT to find the binary
		pluginRoot := filepath.Join(origDir, "..", "..", "..", "..")
		os.Setenv("CLAUDE_PLUGIN_ROOT", pluginRoot)
		defer os.Unsetenv("CLAUDE_PLUGIN_ROOT")

		// Verify binary exists
		binPath := GetGitDiffTreePath()
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			t.Skipf("Binary not found at %s - run 'just build-diff-viz' first", binPath)
		}

		// Get baseline tree SHA
		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		baselineTree := strings.TrimSpace(string(output))

		// Create a file with lots of additions to trip threshold
		testFile := filepath.Join(tmpDir, "big-change.go")
		content := "package main\n\n"
		for i := 0; i < 100; i++ {
			content += "// Line " + string(rune('0'+i%10)) + "\n"
		}
		os.WriteFile(testFile, []byte(content), 0644)

		// Capture the working tree with changes (CaptureTree handles untracked files)
		// Don't use git add -A - CaptureTree handles untracked files via ls-files --others
		currentTree, err := CaptureTree()
		if err != nil {
			t.Fatalf("CaptureTree failed: %v", err)
		}

		// Create session with:
		// - BaselineTree = original (before changes)
		// - PreviousTree = current (simulating PostToolUse having run)
		// - AccumulatedScore over threshold
		sessionID := "test-stop-cumulative"
		sess, err := state.New(sessionID, baselineTree, "main", 50) // Low threshold
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		// Simulate PostToolUse having already updated PreviousTree
		sess.PreviousTree = currentTree
		sess.AccumulatedScore = 100 // Already over 50 threshold
		if err := sess.Save(); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Create lock dir
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		// Capture stdout/stderr to verify response
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		err = Stop(input)

		w.Close()
		os.Stdout = oldStdout

		// Read the output
		var buf [8192]byte
		n, _ := r.Read(buf[:])
		outputStr := string(buf[:n])

		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		// Parse the JSON response
		var resp StopResponse
		if err := json.Unmarshal([]byte(outputStr), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v\nOutput: %s", err, outputStr)
		}

		// Verify it's a block response
		if resp.Decision != "block" {
			t.Errorf("Decision = %q, want 'block'", resp.Decision)
		}

		// Verify cumulative stats are in the response
		// Key check: total additions should be ~100 lines (from BaselineTree diff)
		// If it used PreviousTree=currentTree diff, it would show 0 total additions
		if resp.ThresholdData == nil {
			t.Fatal("ThresholdData is nil")
		}

		thresholdData, ok := resp.ThresholdData.(map[string]interface{})
		if !ok {
			t.Fatalf("ThresholdData is not a map, got %T", resp.ThresholdData)
		}

		// Check total additions (new + edit) - our file has ~100 lines
		newAdds := int(thresholdData["new_additions"].(float64))
		editAdds := int(thresholdData["edit_additions"].(float64))
		totalAdds := newAdds + editAdds

		// Should show cumulative additions from BaselineTree, not 0
		if totalAdds < 50 {
			t.Errorf("total additions (new=%d + edit=%d = %d), want >= 50 (cumulative from BaselineTree)", newAdds, editAdds, totalAdds)
		}

		t.Logf("Breakdown shows new=%d, edit=%d, total=%d (cumulative from BaselineTree)", newAdds, editAdds, totalAdds)
	})
}

func TestStopAllowsWhenUnderThreshold(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("allows stop when under threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Get current tree
		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		currentTree := strings.TrimSpace(string(output))

		// Create session with high threshold (won't be exceeded)
		sessionID := "test-stop-under"
		sess, err := state.New(sessionID, currentTree, "main", 1000)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.AccumulatedScore = 50 // Well under 1000
		sess.Save()

		// Set up checkpoint dir
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		err = Stop(input)

		// Should return nil (no blocking response)
		if err != nil {
			t.Errorf("Stop() under threshold should return nil, got error: %v", err)
		}
	})
}

func TestStopSkipsWhenStopTriggered(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("skips when stop_triggered is true", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		currentTree := strings.TrimSpace(string(output))

		// Create session with stop_triggered = true
		sessionID := "test-stop-triggered"
		sess, err := state.New(sessionID, currentTree, "main", 100)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.StopTriggered = true // Already triggered once
		sess.Save()

		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		err = Stop(input)

		// Should return nil (don't block again)
		if err != nil {
			t.Errorf("Stop() with stop_triggered=true should return nil, got: %v", err)
		}
	})
}

func TestStopSkipsWhenStopHookActive(t *testing.T) {
	t.Run("skips when StopHookActive is true", func(t *testing.T) {
		input := &HookInput{
			SessionID:      "any",
			HookEventName:  "Stop",
			StopHookActive: true, // Already in a Stop hook
		}

		err := Stop(input)

		// Should return nil to prevent infinite loop
		if err != nil {
			t.Errorf("Stop() with StopHookActive=true should return nil, got: %v", err)
		}
	})
}

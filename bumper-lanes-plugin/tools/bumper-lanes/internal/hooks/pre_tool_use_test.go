package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

func TestPreToolUseBlocksWhenStopTriggered(t *testing.T) {
	// This is the critical regression test - PreToolUse must block
	// file modifications when StopTriggered=true AND score still exceeds threshold

	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	sessionID := "test-pretooluse-block"

	// Create initial commit to establish valid baseline
	os.WriteFile("initial.txt", []byte("initial\n"), 0644)
	exec.Command("git", "add", "initial.txt").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	baseline, _ := CaptureTree()

	// Create session with low threshold to ensure dirty changes exceed it
	sess, err := state.New(sessionID, baseline, "main", 50) // Low threshold
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sess.SetStopTriggered(true)
	sess.SetScore(500) // Over threshold (will be recalculated)
	if err := sess.Save(); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Create large uncommitted change to exceed 50pt threshold
	largeContent := make([]byte, 0, 10000)
	for i := 0; i < 100; i++ {
		largeContent = append(largeContent, []byte(fmt.Sprintf("// Line %d\n", i))...)
	}
	os.WriteFile("dirty.txt", largeContent, 0644)

	// Test each file modification tool
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
		t.Run(tool+" blocked when StopTriggered", func(t *testing.T) {
			// Capture stdout to check JSON response
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			input := &HookInput{
				HookEventName: "PreToolUse",
				ToolName:      tool,
				SessionID:     sessionID,
			}

			exitCode := PreToolUse(input)

			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var output []byte
			output = make([]byte, 4096)
			n, _ := r.Read(output)
			output = output[:n]

			// Should exit 0 (JSON API)
			if exitCode != 0 {
				t.Errorf("PreToolUse(%s) exitCode = %d, want 0", tool, exitCode)
			}

			// Should output JSON with permissionDecision: "deny"
			var resp PreToolUseResponse
			if err := json.Unmarshal(output, &resp); err != nil {
				t.Fatalf("Failed to parse JSON response: %v\nOutput: %s", err, output)
			}

			if resp.HookSpecificOutput == nil {
				t.Fatalf("PreToolUse(%s) response missing hookSpecificOutput", tool)
			}

			if resp.HookSpecificOutput.PermissionDecision != "deny" {
				t.Errorf("PreToolUse(%s) permissionDecision = %q, want \"deny\"",
					tool, resp.HookSpecificOutput.PermissionDecision)
			}

			if resp.HookSpecificOutput.PermissionDecisionReason == "" {
				t.Errorf("PreToolUse(%s) should include a reason for denial", tool)
			}
		})
	}
}

func TestPreToolUseAllowsWhenStopNotTriggered(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	sessionID := "test-pretooluse-allow"

	// Create session with StopTriggered=false (under threshold)
	sess, err := state.New(sessionID, "some-baseline", "main", 400)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sess.SetStopTriggered(false)
	sess.SetScore(100) // Under threshold
	if err := sess.Save(); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	input := &HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		SessionID:     sessionID,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := PreToolUse(input)

	w.Close()
	os.Stdout = oldStdout

	var output []byte
	output = make([]byte, 4096)
	n, _ := r.Read(output)
	output = output[:n]

	// Should exit 0 and output nothing (allow)
	if exitCode != 0 {
		t.Errorf("PreToolUse(StopTriggered=false) exitCode = %d, want 0", exitCode)
	}

	// Should NOT output JSON (no blocking)
	if len(output) > 0 {
		t.Errorf("PreToolUse(StopTriggered=false) should not output JSON, got: %s", output)
	}
}

func TestPreToolUseAllowsWhenPaused(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	sessionID := "test-pretooluse-paused"

	// Create session that's paused (even with StopTriggered=true)
	sess, err := state.New(sessionID, "some-baseline", "main", 400)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sess.SetStopTriggered(true)
	sess.Paused = true
	if err := sess.Save(); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	input := &HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		SessionID:     sessionID,
	}

	exitCode := PreToolUse(input)

	// Should allow (paused overrides StopTriggered)
	if exitCode != 0 {
		t.Errorf("PreToolUse(Paused=true) exitCode = %d, want 0", exitCode)
	}
}

func TestPreToolUseAllowsWhenThresholdDisabled(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	sessionID := "test-pretooluse-disabled"

	// Create session with threshold=0 (disabled)
	sess, err := state.New(sessionID, "some-baseline", "main", 0)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sess.SetStopTriggered(true) // Would block if enabled
	if err := sess.Save(); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	input := &HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		SessionID:     sessionID,
	}

	exitCode := PreToolUse(input)

	// Should allow (threshold disabled)
	if exitCode != 0 {
		t.Errorf("PreToolUse(threshold=0) exitCode = %d, want 0", exitCode)
	}
}

func TestPreToolUseIgnoresNonModificationTools(t *testing.T) {
	// PreToolUse should only check Write/Edit/MultiEdit/NotebookEdit
	// Other tools should pass through immediately

	nonModTools := []string{"Read", "Glob", "Grep", "Bash", "Search", "List"}

	for _, tool := range nonModTools {
		t.Run(tool+" passes through", func(t *testing.T) {
			input := &HookInput{
				HookEventName: "PreToolUse",
				ToolName:      tool,
				SessionID:     "any-session",
			}

			exitCode := PreToolUse(input)
			if exitCode != 0 {
				t.Errorf("PreToolUse(%s) = %d, want 0 (pass through)", tool, exitCode)
			}
		})
	}
}

func TestPreToolUseFailsOpenOnMissingSession(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	input := &HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		SessionID:     "nonexistent-session-xyz",
	}

	exitCode := PreToolUse(input)

	// Should fail open (allow tool)
	if exitCode != 0 {
		t.Errorf("PreToolUse(missing session) = %d, want 0 (fail open)", exitCode)
	}
}

func TestPreToolUseWrongHookEvent(t *testing.T) {
	input := &HookInput{
		HookEventName: "PostToolUse", // Wrong event
		ToolName:      "Write",
		SessionID:     "any",
	}

	exitCode := PreToolUse(input)
	if exitCode != 0 {
		t.Errorf("PreToolUse(wrong event) = %d, want 0", exitCode)
	}
}

func TestPreToolUseAutoResetOnCleanTree(t *testing.T) {
	// Skip if not in a git repo
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("auto-resets when tree becomes clean after Stop triggered", func(t *testing.T) {
		// This test verifies the fix for the timing issue:
		// 1. Threshold exceeded → StopTriggered=true
		// 2. User commits externally → tree clean
		// 3. Claude tries Write → PreToolUse should unblock (auto-reset)

		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create initial commit
		os.WriteFile("initial.txt", []byte("initial\n"), 0644)
		exec.Command("git", "add", "initial.txt").Run()
		exec.Command("git", "commit", "-m", "initial").Run()

		// Capture baseline
		baseline, err := CaptureTree()
		if err != nil {
			t.Fatalf("Failed to capture baseline: %v", err)
		}

		// Create session with StopTriggered=true (threshold exceeded)
		sessionID := "test-pretooluse-reset"
		sess, err := state.New(sessionID, baseline, "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.SetStopTriggered(true)
		sess.SetScore(500) // Above threshold
		sess.Save()

		// Simulate changes and external commit
		os.WriteFile("changes.txt", []byte("new changes\n"), 0644)
		exec.Command("git", "add", "changes.txt").Run()
		exec.Command("git", "commit", "-m", "external commit").Run()

		// Verify tree is clean (matches HEAD)
		currentTree, _ := CaptureTree()
		headTree := GetHeadTree()
		if currentTree != headTree {
			t.Fatalf("Setup failed: tree should be clean")
		}

		// Claude tries Write (PreToolUse should auto-reset and allow)
		input := &HookInput{
			HookEventName: "PreToolUse",
			ToolName:      "Write",
			SessionID:     sessionID,
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		exitCode := PreToolUse(input)

		w.Close()
		os.Stdout = oldStdout

		var output []byte
		output = make([]byte, 4096)
		n, _ := r.Read(output)
		output = output[:n]

		// Should allow (exit 0, no JSON denial)
		if exitCode != 0 {
			t.Errorf("Expected allow (exit 0), got %d", exitCode)
		}

		// Should NOT output JSON (no blocking)
		if len(output) > 0 {
			t.Errorf("Should not output blocking JSON, got: %s", output)
		}

		// Verify session was reset
		reloaded, _ := state.Load(sessionID)
		if reloaded.StopTriggered {
			t.Errorf("StopTriggered should be false after auto-reset")
		}
		if reloaded.Score != 0 {
			t.Errorf("Score = %d, want 0 after auto-reset", reloaded.Score)
		}

		// Verify baseline was updated to HEAD
		if reloaded.BaselineTree != headTree {
			t.Errorf("Baseline not updated to HEAD tree")
		}
	})

	t.Run("still blocks when tree is dirty but over threshold", func(t *testing.T) {
		// Verify blocking still works when tree is dirty AND score exceeds threshold
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create initial commit
		os.WriteFile("initial.txt", []byte("initial\n"), 0644)
		exec.Command("git", "add", "initial.txt").Run()
		exec.Command("git", "commit", "-m", "initial").Run()

		baseline, _ := CaptureTree()
		sessionID := "test-pretooluse-block"
		sess, _ := state.New(sessionID, baseline, "main", 100) // Lower threshold to ensure we exceed it
		sess.SetStopTriggered(true)
		sess.Save()

		// Create uncommitted changes large enough to exceed 100pt threshold
		largeContent := make([]byte, 0, 20000)
		for i := 0; i < 200; i++ {
			largeContent = append(largeContent, []byte(fmt.Sprintf("// Line %d of dirty file\n", i))...)
		}
		os.WriteFile("dirty.txt", largeContent, 0644)

		// PreToolUse should block
		input := &HookInput{
			HookEventName: "PreToolUse",
			ToolName:      "Write",
			SessionID:     sessionID,
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		exitCode := PreToolUse(input)

		w.Close()
		os.Stdout = oldStdout

		var output []byte
		output = make([]byte, 4096)
		n, _ := r.Read(output)
		output = output[:n]

		// Should block (exit 0 with JSON denial)
		if exitCode != 0 {
			t.Errorf("Expected exit 0 (JSON output), got %d", exitCode)
		}

		// Should output JSON with denial
		if len(output) == 0 {
			t.Errorf("Should output blocking JSON")
		}

		// StopTriggered should still be true when over threshold
		reloaded, _ := state.Load(sessionID)
		if !reloaded.StopTriggered {
			t.Errorf("StopTriggered should remain true when score exceeds threshold")
		}
	})

	t.Run("auto-recovers when dirty but score drops below threshold", func(t *testing.T) {
		// NEW TEST: Validate auto-recovery when user manually reduces diff below threshold
		// This is the fix for PR #3 - dynamic threshold checking
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create initial commit
		os.WriteFile("initial.txt", []byte("initial\n"), 0644)
		exec.Command("git", "add", "initial.txt").Run()
		exec.Command("git", "commit", "-m", "initial").Run()

		baseline, _ := CaptureTree()
		sessionID := "test-pretooluse-autorecovery"
		sess, _ := state.New(sessionID, baseline, "main", 200) // Threshold: 200 points
		sess.SetStopTriggered(true)                            // Was over threshold
		sess.SetScore(300)                                     // Was 300 points
		sess.Save()

		// Create small uncommitted change (below threshold)
		os.WriteFile("small.txt", []byte("small change\n"), 0644) // ~1 line = ~1 point

		// PreToolUse should auto-recover (not block)
		input := &HookInput{
			HookEventName: "PreToolUse",
			ToolName:      "Write",
			SessionID:     sessionID,
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		exitCode := PreToolUse(input)

		w.Close()
		os.Stdout = oldStdout

		var output []byte
		output = make([]byte, 4096)
		n, _ := r.Read(output)
		output = output[:n]

		// Should allow (exit 0, no JSON denial)
		if exitCode != 0 {
			t.Errorf("Expected allow (exit 0), got %d", exitCode)
		}

		// Should NOT output JSON blocking (no denial)
		if len(output) > 0 {
			// Check if it's a JSON denial
			var resp PreToolUseResponse
			if err := json.Unmarshal(output, &resp); err == nil {
				if resp.HookSpecificOutput != nil && resp.HookSpecificOutput.PermissionDecision == "deny" {
					t.Errorf("Should not block when score drops below threshold")
				}
			}
		}

		// StopTriggered should be cleared
		reloaded, _ := state.Load(sessionID)
		if reloaded.StopTriggered {
			t.Errorf("StopTriggered should be false after auto-recovery (score below threshold)")
		}

		// Score should be updated to fresh calculation
		if reloaded.Score >= 200 {
			t.Errorf("Score should be below threshold after auto-recovery, got %d", reloaded.Score)
		}
	})
}

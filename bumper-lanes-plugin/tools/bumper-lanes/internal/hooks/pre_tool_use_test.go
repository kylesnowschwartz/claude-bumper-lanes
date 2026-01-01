package hooks

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

func TestPreToolUseBlocksWhenStopTriggered(t *testing.T) {
	// This is the critical regression test - PreToolUse must block
	// file modifications when StopTriggered=true

	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	sessionID := "test-pretooluse-block"

	// Create session with StopTriggered=true (threshold was exceeded)
	sess, err := state.New(sessionID, "some-baseline", "main", 400)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sess.SetStopTriggered(true)
	sess.SetScore(500) // Over threshold
	if err := sess.Save(); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

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

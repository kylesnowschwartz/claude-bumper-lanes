package hooks

import (
	"testing"
)

func TestReadInput(t *testing.T) {
	// Test that HookInput struct can be unmarshaled
	input := HookInput{
		SessionID:      "test-123",
		StopHookActive: false,
		ToolName:       "Write",
		HookEventName:  "PostToolUse",
	}

	if input.SessionID != "test-123" {
		t.Errorf("SessionID = %q, want %q", input.SessionID, "test-123")
	}
	if input.ToolName != "Write" {
		t.Errorf("ToolName = %q, want %q", input.ToolName, "Write")
	}
}

func TestStopResponse(t *testing.T) {
	resp := StopResponse{
		Continue:       true,
		SystemMessage:  "test message",
		SuppressOutput: false,
		Decision:       "block",
		Reason:         "test reason",
	}

	if !resp.Continue {
		t.Error("Continue = false, want true")
	}
	if resp.Decision != "block" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "block")
	}
}

func TestIsGitRepo(t *testing.T) {
	// This test runs in a git repo, so should return true
	if !IsGitRepo() {
		t.Skip("Not running in a git repo")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	branch := GetCurrentBranch()
	// Should return something in a git repo
	if branch == "" {
		t.Skip("No branch or detached HEAD")
	}
	t.Logf("Current branch: %s", branch)
}

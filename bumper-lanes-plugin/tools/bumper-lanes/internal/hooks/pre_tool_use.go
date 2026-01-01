package hooks

import (
	"fmt"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// PreToolUseResponse is the JSON response for PreToolUse hooks.
// Uses the modern hookSpecificOutput format for permission decisions.
//
// Exit code 0 with this JSON structure allows Claude Code to parse
// the permission decision properly.
type PreToolUseResponse struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput contains the PreToolUse-specific output fields.
type HookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`                 // "allow", "deny", or "ask"
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"` // Shown to Claude when denied
}

// PreToolUse handles the PreToolUse hook event.
// It blocks file modification tools (Write, Edit, etc.) when the threshold
// has been exceeded and StopTriggered is true.
//
// This is the "hard enforcement" layer - it prevents tools from executing
// entirely, complementing the Stop hook which blocks turn completion.
//
// Returns exit code 0 for JSON output (even when blocking).
func PreToolUse(input *HookInput) (exitCode int) {
	log := logging.New(input.SessionID, "pre_tool_use")

	// Validate hook event
	if input.HookEventName != "PreToolUse" {
		return 0
	}

	// Only block file modification tools
	switch input.ToolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		// Proceed with threshold check
	default:
		return 0
	}

	// Check if git repo
	if !IsGitRepo() {
		return 0
	}

	// Load session state
	sess, err := state.Load(input.SessionID)
	if err != nil {
		log.Warn("failed to load session: %v (failing open)", err)
		return 0 // Fail open
	}

	// If paused, allow tool
	if sess.Paused {
		return 0
	}

	// If threshold is 0 (disabled), allow tool
	if sess.ThresholdLimit == 0 {
		return 0
	}

	// KEY CHECK: Only block if Stop hook has already triggered
	// This ensures we don't prematurely block before the user sees the threshold warning
	if !sess.StopTriggered {
		return 0
	}

	// Stop was triggered and not reset - block the tool
	pct := 0
	if sess.ThresholdLimit > 0 {
		pct = (sess.Score * 100) / sess.ThresholdLimit
	}

	reason := formatBlockReason(sess.Score, sess.ThresholdLimit, pct)

	resp := PreToolUseResponse{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		},
	}

	if err := WriteResponse(resp); err != nil {
		log.Warn("failed to write response: %v", err)
	}

	return 0 // Exit 0 for JSON output
}

// formatBlockReason creates the denial message shown to Claude.
func formatBlockReason(score, limit, pct int) string {
	return `Bumper lanes: File modifications blocked.

Threshold exceeded: ` + formatScore(score, limit, pct) + `

The Stop hook has already fired. To continue:
1. Review the changes with the user
2. Run /bumper-reset to restore budget

This prevents unbounded changes without review.`
}

// formatScore formats the score display.
func formatScore(score, limit, pct int) string {
	return fmt.Sprintf("%d/%d pts (%d%%)", score, limit, pct)
}

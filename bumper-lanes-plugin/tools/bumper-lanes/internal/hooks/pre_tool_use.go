package hooks

import (
	"fmt"
	"os"

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
// NEW (v3.7.0): Before blocking, checks if working tree has become clean
// (matches HEAD) since Stop hook triggered. If clean, auto-resets baseline
// and clears StopTriggered flag, allowing the tool to proceed.
//
// This handles external commits (IDE, terminal) that clean the tree between
// Stop hook firing and the next Write/Edit attempt.
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

	// ╔═══════════════════════════════════════════════════════════╗
	// ║ AUTO-RESET: Check if tree has become clean since Stop    ║
	// ║ This handles external commits before Claude writes       ║
	// ╚═══════════════════════════════════════════════════════════╝
	// Check if tree has become clean since Stop hook triggered
	// This handles external commits (IDE, terminal, git CLI) that clean the tree
	if sess.StopTriggered {
		currentTree, err := CaptureTree()
		if err == nil {
			headTree := GetHeadTree()
			if headTree != "" && currentTree == headTree {
				// Tree is clean - auto-reset baseline and clear flag
				currentBranch := GetCurrentBranch()
				sess.ResetBaseline(currentTree, currentBranch)
				sess.Save()

				// Provide feedback to user and Claude
				fmt.Fprintf(os.Stderr, "✓ Baseline auto-reset (external commit detected). Budget restored.\n")
				return 0
			}
		}
		// Tree is dirty or check failed - fall through to blocking
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
1. Review changes with the user
2. Commit changes (baseline auto-resets), OR
3. Run /bumper-reset to manually restore budget

This prevents unbounded changes without review.`
}

// formatScore formats the score display.
func formatScore(score, limit, pct int) string {
	return fmt.Sprintf("%d/%d pts (%d%%)", score, limit, pct)
}

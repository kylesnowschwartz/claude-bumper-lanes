// Package hookio implements the Claude Code hook wire protocol: the JSON
// payloads hooks receive on stdin and the response shapes they write to
// stdout. Pure encode/decode - no enforcement logic lives here.
package hookio

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
)

// Input represents the JSON input from Claude Code hooks.
// Field presence varies by event and source (e.g. SessionStart with
// source "resume" omits fields that "startup" includes) - parse defensively.
type Input struct {
	SessionID      string     `json:"session_id"`
	StopHookActive bool       `json:"stop_hook_active,omitempty"`
	ToolName       string     `json:"tool_name,omitempty"`
	HookEventName  string     `json:"hook_event_name,omitempty"`
	Source         string     `json:"source,omitempty"` // SessionStart: startup|resume|clear|compact
	ToolInput      *ToolInput `json:"tool_input,omitempty"`
	UserPrompt     string     `json:"user_prompt,omitempty"` // For UserPromptSubmit hooks
	Prompt         string     `json:"prompt,omitempty"`      // Alternative field name
}

// GetPrompt returns the user prompt, checking both field names.
func (h *Input) GetPrompt() string {
	if h.UserPrompt != "" {
		return h.UserPrompt
	}
	return h.Prompt
}

// ToolInput contains the input for a tool invocation.
type ToolInput struct {
	Command string `json:"command,omitempty"` // For Bash tool
}

// ContextResponse injects additionalContext into Claude's context.
// Supported by SessionStart, UserPromptSubmit, and PostToolUse.
type ContextResponse struct {
	HookSpecificOutput ContextOutput `json:"hookSpecificOutput"`
}

// ContextOutput is the event-specific payload of a ContextResponse.
type ContextOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// StopResponse is the JSON response for Stop hooks.
//
// Claude Code Stop hook semantics are counterintuitive:
//   - Continue: true = Claude keeps working, false = Claude stops entirely
//   - Decision: "block" = block the STOP (keeps Claude working), not block Claude
//   - continue: false takes precedence over decision: "block"
//
// See the Stop handler in internal/hooks for a detailed explanation.
type StopResponse struct {
	Continue         bool        `json:"continue"`                   // true=Claude continues, false=Claude stops
	SystemMessage    string      `json:"systemMessage,omitempty"`    // Injected into Claude's context
	SuppressOutput   bool        `json:"suppressOutput,omitempty"`   // Hide Claude's pending output
	Decision         string      `json:"decision,omitempty"`         // "block" = block the stop (not Claude!)
	Reason           string      `json:"reason,omitempty"`           // Shown to user when blocking
	TerminalSequence string      `json:"terminalSequence,omitempty"` // Escape sequence written to the terminal (e.g. OSC 9 desktop notification)
	ThresholdData    interface{} `json:"threshold_data,omitempty"`   // Custom data for debugging
}

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

// UserPromptResponse is the JSON structure for UserPromptSubmit hook output.
// decision="block" + reason="message" shows output to user without API call.
type UserPromptResponse struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Read reads and parses hook JSON input from stdin.
// A failure here is logged before the caller fails open: unparseable input
// is otherwise invisible (exit 0, no output), indistinguishable from a
// healthy no-op. Only the error and payload size are logged, never the
// payload - tool_input can carry file contents.
func Read() (*Input, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		logging.New("unparsed-input", "read-input").Warn("reading stdin failed: %v (failing open)", err)
		return nil, fmt.Errorf("reading stdin: %w", err)
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		logging.New("unparsed-input", "read-input").Warn("invalid hook JSON (%d bytes): %v (failing open)", len(data), err)
		return nil, fmt.Errorf("parsing input: %w", err)
	}

	return &input, nil
}

// Write writes JSON response to stdout.
func Write(resp interface{}) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// WriteContext emits a ContextResponse for the given event to stdout.
func WriteContext(eventName, context string) error {
	return Write(ContextResponse{
		HookSpecificOutput: ContextOutput{
			HookEventName:     eventName,
			AdditionalContext: context,
		},
	})
}

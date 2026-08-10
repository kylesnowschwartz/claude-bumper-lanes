// Package hooks provides common functionality for bumper-lanes hook handlers.
package hooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// HookInput represents the JSON input from Claude Code hooks.
// Field presence varies by event and source (e.g. SessionStart with
// source "resume" omits fields that "startup" includes) - parse defensively.
type HookInput struct {
	SessionID      string     `json:"session_id"`
	StopHookActive bool       `json:"stop_hook_active,omitempty"`
	ToolName       string     `json:"tool_name,omitempty"`
	HookEventName  string     `json:"hook_event_name,omitempty"`
	Source         string     `json:"source,omitempty"` // SessionStart: startup|resume|clear|compact
	ToolInput      *ToolInput `json:"tool_input,omitempty"`
	UserPrompt     string     `json:"user_prompt,omitempty"` // For UserPromptSubmit hooks
	Prompt         string     `json:"prompt,omitempty"`      // Alternative field name

	// Tool execution result (PostToolUse). Encoded as a string (tool_result)
	// or a string/object (tool_response) depending on Claude Code version.
	ToolResult   json.RawMessage `json:"tool_result,omitempty"`
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
}

// ToolResultText returns the tool's execution result as plain text,
// tolerating both the tool_result and tool_response encodings.
// Returns "" when neither field is present.
func (h *HookInput) ToolResultText() string {
	for _, raw := range []json.RawMessage{h.ToolResult, h.ToolResponse} {
		if len(raw) == 0 {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return string(raw)
	}
	return ""
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

// WriteContext emits a ContextResponse for the given event to stdout.
func WriteContext(eventName, context string) error {
	return WriteResponse(ContextResponse{
		HookSpecificOutput: ContextOutput{
			HookEventName:     eventName,
			AdditionalContext: context,
		},
	})
}

// GetPrompt returns the user prompt, checking both field names.
func (h *HookInput) GetPrompt() string {
	if h.UserPrompt != "" {
		return h.UserPrompt
	}
	return h.Prompt
}

// ToolInput contains the input for a tool invocation.
type ToolInput struct {
	Command string `json:"command,omitempty"` // For Bash tool
}

// StopResponse is the JSON response for Stop hooks.
//
// Claude Code Stop hook semantics are counterintuitive:
//   - Continue: true = Claude keeps working, false = Claude stops entirely
//   - Decision: "block" = block the STOP (keeps Claude working), not block Claude
//   - continue: false takes precedence over decision: "block"
//
// See stop.go for detailed explanation of these semantics.
type StopResponse struct {
	Continue       bool        `json:"continue"`                 // true=Claude continues, false=Claude stops
	SystemMessage  string      `json:"systemMessage,omitempty"`  // Injected into Claude's context
	SuppressOutput bool        `json:"suppressOutput,omitempty"` // Hide Claude's pending output
	Decision       string      `json:"decision,omitempty"`       // "block" = block the stop (not Claude!)
	Reason         string      `json:"reason,omitempty"`         // Shown to user when blocking
	ThresholdData  interface{} `json:"threshold_data,omitempty"` // Custom data for debugging
}

// ReadInput reads and parses hook JSON input from stdin.
func ReadInput() (*HookInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}

	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing input: %w", err)
	}

	return &input, nil
}

// WriteResponse writes JSON response to stdout.
func WriteResponse(resp interface{}) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// IsGitRepo checks if current directory is in a git repository.
func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// CaptureTree captures the current working tree as a git tree SHA.
// Uses a temporary index to avoid modifying the real staging area.
func CaptureTree() (string, error) {
	// Create temp index file
	tmpIndex, err := os.CreateTemp("", "git-index-*")
	if err != nil {
		return "", err
	}
	tmpIndexPath := tmpIndex.Name()
	tmpIndex.Close()
	defer os.Remove(tmpIndexPath)

	// Helper to run git commands with GIT_INDEX_FILE set
	gitWithTempIndex := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndexPath)
		return cmd
	}

	// Initialize temp index with HEAD tree (or empty if no commits)
	headRef, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil && len(headRef) > 0 {
		gitWithTempIndex("read-tree", strings.TrimSpace(string(headRef))).Run()
	} else {
		gitWithTempIndex("read-tree", "--empty").Run()
	}

	// Add tracked file changes (staged and unstaged)
	gitWithTempIndex("add", "-u", ".").Run()

	// Add untracked files (respecting .gitignore)
	lsCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedOutput, _ := lsCmd.Output()
	if len(untrackedOutput) > 0 {
		scanner := bufio.NewScanner(bytes.NewReader(untrackedOutput))
		for scanner.Scan() {
			path := scanner.Text()
			if path != "" {
				gitWithTempIndex("add", path).Run()
			}
		}
	}

	// Write tree from temp index
	writeCmd := gitWithTempIndex("write-tree")
	output, err := writeCmd.Output()
	if err != nil {
		return "", err
	}

	treeSHA := strings.TrimSpace(string(output))
	if treeSHA == "" {
		return "", fmt.Errorf("empty tree SHA")
	}

	return treeSHA, nil
}

// GetCurrentBranch returns the current branch name, or empty string if detached.
func GetCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(output))
	if branch == "HEAD" {
		return "" // Detached HEAD
	}
	return branch
}

// GetHeadTree returns the tree SHA of HEAD.
// Returns empty string if HEAD doesn't exist (empty repo) or on error.
func GetHeadTree() string {
	cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

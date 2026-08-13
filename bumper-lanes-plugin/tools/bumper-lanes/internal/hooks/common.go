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

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
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
	Continue         bool        `json:"continue"`                   // true=Claude continues, false=Claude stops
	SystemMessage    string      `json:"systemMessage,omitempty"`    // Injected into Claude's context
	SuppressOutput   bool        `json:"suppressOutput,omitempty"`   // Hide Claude's pending output
	Decision         string      `json:"decision,omitempty"`         // "block" = block the stop (not Claude!)
	Reason           string      `json:"reason,omitempty"`           // Shown to user when blocking
	TerminalSequence string      `json:"terminalSequence,omitempty"` // Escape sequence written to the terminal (e.g. OSC 9 desktop notification)
	ThresholdData    interface{} `json:"threshold_data,omitempty"`   // Custom data for debugging
}

// ReadInput reads and parses hook JSON input from stdin.
// A failure here is logged before the caller fails open: unparseable input
// is otherwise invisible (exit 0, no output), indistinguishable from a
// healthy no-op. Only the error and payload size are logged, never the
// payload - tool_input can carry file contents.
func ReadInput() (*HookInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		logging.New("unparsed-input", "read-input").Warn("reading stdin failed: %v (failing open)", err)
		return nil, fmt.Errorf("reading stdin: %w", err)
	}

	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		logging.New("unparsed-input", "read-input").Warn("invalid hook JSON (%d bytes): %v (failing open)", len(data), err)
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
		if err := gitWithTempIndex("read-tree", strings.TrimSpace(string(headRef))).Run(); err != nil {
			return "", fmt.Errorf("read-tree HEAD: %w", err)
		}
	} else {
		if err := gitWithTempIndex("read-tree", "--empty").Run(); err != nil {
			return "", fmt.Errorf("read-tree --empty: %w", err)
		}
	}

	// Add tracked file changes (staged and unstaged). A repo with no
	// tracked files yet (fresh init, or HEAD^{tree} empty) reports a
	// pathspec error here - benign, since there is nothing to update.
	if out, err := gitWithTempIndex("add", "-u", ".").CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "did not match any file") {
			return "", fmt.Errorf("add -u: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}

	// Add untracked files (respecting .gitignore)
	lsCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedOutput, err := lsCmd.Output()
	if err != nil {
		return "", fmt.Errorf("ls-files --others: %w", err)
	}
	if len(untrackedOutput) > 0 {
		scanner := bufio.NewScanner(bytes.NewReader(untrackedOutput))
		for scanner.Scan() {
			path := scanner.Text()
			if path != "" {
				if err := gitWithTempIndex("add", path).Run(); err != nil {
					return "", fmt.Errorf("add %q: %w", path, err)
				}
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

// saveOrLog saves session state, logging a WARN naming the caller's context
// on failure. The operation that called it proceeds regardless (fail open).
func saveOrLog(sess *state.SessionState, log *logging.Logger, context string) {
	if err := sess.Save(); err != nil {
		log.Warn("failed to save session state (%s): %v (failing open)", context, err)
	}
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

// GetHeadCommit returns the commit SHA of HEAD, or "none" on an unborn
// branch (fresh repo with no commits). The non-empty sentinel lets callers
// distinguish "recorded on an unborn branch" from "never recorded".
func GetHeadCommit() string {
	output, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "none"
	}
	return strings.TrimSpace(string(output))
}

// GetHeadTree returns the tree SHA of HEAD.
// Returns empty string if HEAD doesn't exist (empty repo) or on error.
func GetHeadTree() string {
	return revParseTree("HEAD")
}

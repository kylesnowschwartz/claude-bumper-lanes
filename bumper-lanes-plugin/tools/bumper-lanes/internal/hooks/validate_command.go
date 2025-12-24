package hooks

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// viewCmdPattern matches /bumper-view or /claude-bumper-lanes:bumper-view
var viewCmdPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-view\s*(.*)$`)

// UserPromptResponse is the JSON structure for UserPromptSubmit hook output.
// For UserPromptSubmit: decision="block" + reason="message" shows to user.
type UserPromptResponse struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// ValidateCommand validates slash commands before execution.
// Uses JSON output to stdout with exit 0.
// For UserPromptSubmit: decision="block" + reason="message" shows to user.
func ValidateCommand(input *HookInput) int {
	prompt := strings.TrimSpace(input.GetPrompt())
	if prompt == "" {
		return 0
	}

	matches := viewCmdPattern.FindStringSubmatch(prompt)
	if matches == nil {
		return 0
	}

	args := strings.TrimSpace(matches[1])
	if args == "" {
		// No args - block prompt and show usage hint to user
		resp := UserPromptResponse{
			Decision: "block",
			Reason:   "Use `/bumper-view <mode>` to change. Modes: tree, collapsed, smart, topn, icicle, brackets",
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}

	return 0
}

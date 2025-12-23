package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// UserPromptInput is the JSON input for user-prompt-submit hook.
type UserPromptInput struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id"`
}

// UserPromptOutput is the JSON output for user-prompt-submit hook.
// Uses additionalContext to let Claude acknowledge (triggering status line refresh).
type UserPromptOutput struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput contains context added to the prompt.
type HookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// Command patterns
const (
	cmdReset  = "/claude-bumper-lanes:bumper-reset"
	cmdPause  = "/claude-bumper-lanes:bumper-pause"
	cmdResume = "/claude-bumper-lanes:bumper-resume"
	cmdView   = "/claude-bumper-lanes:bumper-view"
	cmdConfig = "/claude-bumper-lanes:bumper-config"
)

// Regex patterns for extracting arguments
// These match both inline args and "Additional user arguments:" from command expansion
var (
	viewModePattern       = regexp.MustCompile(`/claude-bumper-lanes:bumper-view\s+([a-z]+)`)
	viewModeArgsPattern   = regexp.MustCompile(`Additional user arguments:\s*([a-z]+)`)
	configArgsPattern     = regexp.MustCompile(`/claude-bumper-lanes:bumper-config\s+([a-z]+)(?:\s+(\d+))?`)
	configArgsAltPattern  = regexp.MustCompile(`Additional user arguments:\s*([a-z]+)(?:\s+(\d+))?`)
)


// UserPromptSubmitFromStdin reads stdin directly and handles the hook.
// This is separate from the common hook flow because user-prompt-submit
// needs the "prompt" field which other hooks don't use.
func UserPromptSubmitFromStdin() int {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 0 // Fail open
	}

	var input UserPromptInput
	if err := json.Unmarshal(data, &input); err != nil {
		return 0 // Fail open
	}

	// All commands now use bash execution via command files
	// Hook handlers left in place for reference only

	// /bumper-reset - DISABLED: using bash execution instead
	// if strings.Contains(prompt, cmdReset) {
	// 	output := captureOutput(func() error {
	// 		return Reset(sessionID)
	// 	})
	// 	outputCommandResult(output)
	// 	return 0
	// }

	// /bumper-pause - DISABLED: using bash execution instead
	// if strings.Contains(prompt, cmdPause) {
	// 	output := captureOutput(func() error {
	// 		return Pause(sessionID)
	// 	})
	// 	outputCommandResult(output)
	// 	return 0
	// }

	// /bumper-resume - DISABLED: using bash execution instead
	// if strings.Contains(prompt, cmdResume) {
	// 	output := captureOutput(func() error {
	// 		return Resume(sessionID)
	// 	})
	// 	outputCommandResult(output)
	// 	return 0
	// }

	// /bumper-view <mode> - DISABLED: testing if !`bash` syntax works with $ARGUMENTS
	// if strings.Contains(prompt, cmdView) {
	// 	mode := ""
	// 	// Try inline pattern first: /bumper-view tree
	// 	if matches := viewModePattern.FindStringSubmatch(prompt); len(matches) > 1 {
	// 		mode = matches[1]
	// 	}
	// 	// Fall back to command expansion: "Additional user arguments: tree"
	// 	if mode == "" {
	// 		if matches := viewModeArgsPattern.FindStringSubmatch(prompt); len(matches) > 1 {
	// 			mode = matches[1]
	// 		}
	// 	}
	// 	output := captureOutput(func() error {
	// 		return View(sessionID, mode)
	// 	})
	// 	outputCommandResult(output)
	// 	return 0
	// }

	// /bumper-config [action] [value] - DISABLED: using bash execution instead
	// if strings.Contains(prompt, cmdConfig) {
	// 	action := "show"
	// 	value := ""
	// 	// Try inline pattern first: /bumper-config set 300
	// 	if matches := configArgsPattern.FindStringSubmatch(prompt); len(matches) > 1 {
	// 		action = matches[1]
	// 		if len(matches) > 2 {
	// 			value = matches[2]
	// 		}
	// 	}
	// 	// Fall back to command expansion: "Additional user arguments: set 300"
	// 	if action == "show" {
	// 		if matches := configArgsAltPattern.FindStringSubmatch(prompt); len(matches) > 1 {
	// 			action = matches[1]
	// 			if len(matches) > 2 {
	// 				value = matches[2]
	// 			}
	// 		}
	// 	}
	//
	// 	output := captureOutput(func() error {
	// 		switch action {
	// 		case "show":
	// 			return ConfigShow()
	// 		case "set":
	// 			if value == "" {
	// 				return fmt.Errorf("usage: /bumper-config set <value>")
	// 			}
	// 			return ConfigSet(value)
	// 		case "personal":
	// 			if value == "" {
	// 				return fmt.Errorf("usage: /bumper-config personal <value>")
	// 			}
	// 			return ConfigPersonal(value)
	// 		default:
	// 			return fmt.Errorf("unknown config action: %s", action)
	// 		}
	// 	})
	// 	outputCommandResult(output)
	// 	return 0
	// }

	// No command matched - silent success
	return 0
}

// captureOutput runs a function and captures its stdout/stderr output.
func captureOutput(fn func() error) string {
	// Capture stdout
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	err := fn()

	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	output := buf.String()
	if err != nil {
		if output != "" {
			output += "\n"
		}
		output += fmt.Sprintf("error: %v", err)
	}

	return strings.TrimSpace(output)
}

// outputCommandResult outputs JSON with additionalContext.
// Claude sees this and acknowledges briefly, which triggers status line refresh.
func outputCommandResult(output string) {
	// Format: checkmark + result, tells Claude command already executed
	context := fmt.Sprintf("✓ %s\n\nThis command was executed by a hook. Acknowledge with just: ✓", output)

	result := UserPromptOutput{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     "UserPromptSubmit",
			AdditionalContext: context,
		},
	}
	data, _ := json.Marshal(result)
	fmt.Println(string(data))
}

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
type UserPromptOutput struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
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
var (
	viewModePattern   = regexp.MustCompile(`/claude-bumper-lanes:bumper-view\s+([a-z]+)`)
	configArgsPattern = regexp.MustCompile(`/claude-bumper-lanes:bumper-config\s+([a-z]+)(?:\s+(\d+))?`)
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

	prompt := input.Prompt
	sessionID := input.SessionID

	// Check for commands in order of specificity

	// /bumper-reset
	if strings.Contains(prompt, cmdReset) {
		output := captureOutput(func() error {
			return Reset(sessionID)
		})
		outputCommandResult(output)
		return 0
	}

	// /bumper-pause
	if strings.Contains(prompt, cmdPause) {
		output := captureOutput(func() error {
			return Pause(sessionID)
		})
		outputCommandResult(output)
		return 0
	}

	// /bumper-resume
	if strings.Contains(prompt, cmdResume) {
		output := captureOutput(func() error {
			return Resume(sessionID)
		})
		outputCommandResult(output)
		return 0
	}

	// /bumper-view <mode>
	if strings.Contains(prompt, cmdView) {
		mode := ""
		if matches := viewModePattern.FindStringSubmatch(prompt); len(matches) > 1 {
			mode = matches[1]
		}
		output := captureOutput(func() error {
			return View(sessionID, mode)
		})
		outputCommandResult(output)
		return 0
	}

	// /bumper-config [action] [value]
	if strings.Contains(prompt, cmdConfig) {
		action := "show"
		value := ""
		if matches := configArgsPattern.FindStringSubmatch(prompt); len(matches) > 1 {
			action = matches[1]
			if len(matches) > 2 {
				value = matches[2]
			}
		}

		output := captureOutput(func() error {
			switch action {
			case "show":
				return ConfigShow()
			case "set":
				if value == "" {
					return fmt.Errorf("usage: /bumper-config set <value>")
				}
				return ConfigSet(value)
			case "personal":
				if value == "" {
					return fmt.Errorf("usage: /bumper-config personal <value>")
				}
				return ConfigPersonal(value)
			default:
				return fmt.Errorf("unknown config action: %s", action)
			}
		})
		outputCommandResult(output)
		return 0
	}

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

// outputCommandResult outputs the hookSpecificOutput JSON.
func outputCommandResult(output string) {
	result := UserPromptOutput{}
	result.HookSpecificOutput.HookEventName = "UserPromptSubmit"
	result.HookSpecificOutput.AdditionalContext = output

	data, _ := json.Marshal(result)
	fmt.Println(string(data))
}

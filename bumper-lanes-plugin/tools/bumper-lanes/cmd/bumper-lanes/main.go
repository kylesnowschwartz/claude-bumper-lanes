// Command bumper-lanes is the unified hook handler for bumper-lanes.
// It handles all hook events and user commands via subcommands.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hooks"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/statusline"
)

const usage = `bumper-lanes - Threshold enforcement for Claude Code

Usage:
  bumper-lanes <command> [args]

Hook Commands (called by hooks.json):
  session-start       Initialize session state
  pre-tool-use        Block Write/Edit when threshold exceeded
  post-tool-use       Fuel gauge warnings after Write/Edit
  stop                Threshold enforcement check
  session-end         Cleanup session state

User Commands (called via bash in command files):
  reset <session>   Reset baseline after review
  pause <session>   Temporarily disable enforcement
  resume <session>  Re-enable enforcement
  config            Show/set threshold configuration
  budget [session]  Print remaining review budget (latest session if omitted)
  review-clear [session]  Clear a tripped breaker after self-review (on_trip: review)

Status Line Widget:
  status [--widget=TYPE]  Output bumper-lanes status (reads JSON from stdin)
                          Types: all (default), indicator
                          Use --widget=indicator for just the threshold gauge
`

func main() {
	// No args: default to status command (for statusLine.command usage)
	if len(os.Args) < 2 {
		if err := cmdStatus(nil); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	var exitCode int

	switch cmd {
	case "session-start":
		exitCode = cmdSessionStart()
	case "pre-tool-use":
		exitCode = cmdPreToolUse()
	case "post-tool-use":
		exitCode = cmdPostToolUse()
	case "stop":
		err = cmdStop()
	case "session-end":
		err = cmdSessionEnd()
	case "reset":
		err = cmdReset(args)
	case "pause":
		err = cmdPause(args)
	case "resume":
		err = cmdResume(args)
	case "config":
		err = cmdConfig(args)
	case "budget":
		err = cmdBudget(args)
	case "review-clear":
		err = cmdReviewClear(args)
	case "status":
		err = cmdStatus(args)
	case "handle-prompt":
		exitCode = cmdHandlePrompt()
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// Hook command implementations

func cmdSessionStart() int {
	input, err := hooks.ReadInput()
	if err != nil {
		return 0 // Fail open
	}
	return hooks.SessionStart(input)
}

func cmdPreToolUse() int {
	input, err := hooks.ReadInput()
	if err != nil {
		return 0 // Fail open
	}
	return hooks.PreToolUse(input)
}

func cmdPostToolUse() int {
	input, err := hooks.ReadInput()
	if err != nil {
		return 0 // Fail open
	}
	return hooks.PostToolUse(input)
}

func cmdStop() error {
	input, err := hooks.ReadInput()
	if err != nil {
		return nil // Fail open
	}
	return hooks.Stop(input)
}

func cmdSessionEnd() error {
	input, err := hooks.ReadInput()
	if err != nil {
		return nil // Fail open
	}
	return hooks.SessionEnd(input)
}

// User command implementations

func cmdReset(args []string) error {
	sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if len(args) >= 1 {
		sessionID = args[0]
	}
	if sessionID == "" {
		return fmt.Errorf("no session_id: set CLAUDE_CODE_SESSION_ID or pass as arg")
	}
	return hooks.Reset(sessionID)
}

func cmdPause(args []string) error {
	sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if len(args) >= 1 {
		sessionID = args[0]
	}
	if sessionID == "" {
		return fmt.Errorf("no session_id: set CLAUDE_CODE_SESSION_ID or pass as arg")
	}
	return hooks.Pause(sessionID)
}

func cmdResume(args []string) error {
	sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if len(args) >= 1 {
		sessionID = args[0]
	}
	if sessionID == "" {
		return fmt.Errorf("no session_id: set CLAUDE_CODE_SESSION_ID or pass as arg")
	}
	return hooks.Resume(sessionID)
}

func cmdConfig(args []string) error {
	if len(args) == 0 || args[0] == "show" {
		return hooks.ConfigShow()
	}
	if args[0] == "set" && len(args) >= 2 {
		return hooks.ConfigSet(args[1])
	}
	return fmt.Errorf("usage: bumper-lanes config [show|set <value>]")
}

func cmdBudget(args []string) error {
	sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if len(args) >= 1 {
		sessionID = args[0]
	}
	// Empty sessionID is fine: Budget falls back to the latest session.
	return hooks.Budget(sessionID)
}

func cmdReviewClear(args []string) error {
	sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if len(args) >= 1 {
		sessionID = args[0]
	}
	// Empty sessionID is fine: ReviewClear falls back to the latest session.
	return hooks.ReviewClear(sessionID)
}

// Prompt handler (UserPromptSubmit hook)

func cmdHandlePrompt() int {
	input, err := hooks.ReadInput()
	if err != nil {
		return 0 // Fail open
	}
	return hooks.HandlePrompt(input)
}

// Status line widget command

func cmdStatus(args []string) error {
	// Parse --widget flag
	widget := statusline.WidgetAll
	for i, arg := range args {
		if strings.HasPrefix(arg, "--widget=") {
			widget = strings.TrimPrefix(arg, "--widget=")
		} else if arg == "--widget" && i+1 < len(args) {
			widget = args[i+1]
		}
	}

	// Read JSON from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	input, err := statusline.ParseInput(data)
	if err != nil {
		return err
	}

	// Pre-v4 wrappers still ask for the removed diff-tree widget: output
	// nothing without paying for a render.
	if widget == "diff-tree" {
		return nil
	}

	// The indicator widget skips the model/branch/cost work entirely.
	render := statusline.Render
	if widget == statusline.WidgetIndicator {
		render = statusline.RenderIndicator
	}
	output, err := render(input)
	if err != nil {
		return err
	}

	// Output the formatted widget
	fmt.Print(statusline.FormatOutput(output, widget))
	return nil
}

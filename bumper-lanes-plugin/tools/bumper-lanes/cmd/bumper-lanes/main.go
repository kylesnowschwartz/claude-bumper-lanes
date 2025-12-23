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
  post-tool-use       Fuel gauge warnings after Write/Edit
  stop                Threshold enforcement check
  session-end         Cleanup session state

User Commands (called via bash in command files):
  reset <session>   Reset baseline after review
  pause <session>   Temporarily disable enforcement
  resume <session>  Re-enable enforcement
  view <session>    Set visualization mode
  config            Show/set threshold configuration

Status Line Widget:
  status              Output bumper-lanes status (reads JSON from stdin)
                      Pipe Claude Code status JSON to get formatted widget output
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	var exitCode int

	switch cmd {
	case "session-start":
		err = cmdSessionStart()
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
	case "view":
		err = cmdView(args)
	case "config":
		err = cmdConfig(args)
	case "status":
		err = cmdStatus()
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

func cmdSessionStart() error {
	input, err := hooks.ReadInput()
	if err != nil {
		return nil // Fail open
	}
	return hooks.SessionStart(input)
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

func cmdView(args []string) error {
	sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	mode := ""
	var opts []string

	// Parse args: first non-flag arg is mode, rest are flags
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			opts = append(opts, arg)
		} else if mode == "" {
			mode = arg
		} else {
			// Could be flag value (e.g., "100" after "--width")
			opts = append(opts, arg)
		}
	}

	if sessionID == "" {
		return fmt.Errorf("no session_id: set CLAUDE_CODE_SESSION_ID or pass as arg")
	}
	if mode == "" {
		return fmt.Errorf("usage: bumper-lanes view <mode> [--width N] [--depth N]")
	}

	optsStr := strings.Join(opts, " ")
	return hooks.View(sessionID, mode, optsStr)
}

func cmdConfig(args []string) error {
	if len(args) == 0 || args[0] == "show" {
		return hooks.ConfigShow()
	}
	if args[0] == "set" && len(args) >= 2 {
		return hooks.ConfigSet(args[1])
	}
	if args[0] == "personal" && len(args) >= 2 {
		return hooks.ConfigPersonal(args[1])
	}
	return fmt.Errorf("usage: bumper-lanes config [show|set <value>|personal <value>]")
}

// Status line widget command

func cmdStatus() error {
	// Read JSON from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	input, err := statusline.ParseInput(data)
	if err != nil {
		return err
	}

	output, err := statusline.Render(input)
	if err != nil {
		return err
	}

	// Output the formatted widget
	fmt.Print(statusline.FormatOutput(output))
	return nil
}

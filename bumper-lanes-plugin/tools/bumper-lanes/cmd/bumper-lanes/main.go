// Command bumper-lanes is the unified hook handler for bumper-lanes.
// It handles all hook events and user commands via subcommands.
package main

import (
	"fmt"
	"os"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hooks"
)

const usage = `bumper-lanes - Threshold enforcement for Claude Code

Usage:
  bumper-lanes <command> [args]

Hook Commands (called by hooks.json):
  session-start     Initialize session state
  post-tool-use     Fuel gauge warnings after Write/Edit
  stop              Threshold enforcement check
  session-end       Cleanup session state

User Commands (called via user-prompt-submit):
  reset <session>   Reset baseline after review
  pause <session>   Temporarily disable enforcement
  resume <session>  Re-enable enforcement
  view <session>    Set visualization mode
  config            Show/set threshold configuration
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
	if len(args) < 1 {
		return fmt.Errorf("usage: bumper-lanes reset <session_id>")
	}
	return hooks.Reset(args[0])
}

func cmdPause(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bumper-lanes pause <session_id>")
	}
	return hooks.Pause(args[0])
}

func cmdResume(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bumper-lanes resume <session_id>")
	}
	return hooks.Resume(args[0])
}

func cmdView(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: bumper-lanes view <session_id> <mode>")
	}
	return hooks.View(args[0], args[1])
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

# Claude Bumper Lanes

**Purpose**: Enforce incremental code review by blocking Claude Code file modifications beyond configurable diff thresholds.

## Architecture Overview

Defense-in-depth hook system with three enforcement layers:

1. **Proactive Block** (PreToolUse): Deny `Write`/`Edit` tools before execution when threshold exceeded
2. **Reactive Block** (Stop): Prevent Claude from finishing turn, force user notification
3. **Manual Reset** (UserPromptSubmit): Intercept `/claude-bumper-lanes:bumper-reset` command to restore budget after review

## Technology Stack

- **Bash 4.0+**: Hook scripts and state management
- **Git 2.x+**: Working tree snapshots via `git write-tree`, diff calculation via `git diff-tree`
- **jq**: JSON parsing for hook I/O and session state
- **Claude Code Hooks**: SessionStart, PreToolUse, Stop, UserPromptSubmit events

## Design Principles

- **Stateful enforcement**: Track cumulative diff per session against baseline snapshot
- **Fail-open**: Errors allow operations (availability over strictness)
- **Explicit approval**: User must manually reset after reviewing changes
- **Transparent feedback**: Both user and Claude see threshold status and reasons

## Key Implementation Details

- Default threshold: 200 lines changed (additions + deletions)
- Session state persisted in `~/.cache/claude-code/bumper-lanes/{session_id}.json`
- Baseline reset captures current `git write-tree` SHA as new reference point
- PreToolUse matcher: `Edit|Write` (Bash tool not blocked - allows read-only commands)
- Stop hook exit code 2 blocks Claude from finishing cleanly

## Project Structure

```
bumper-lanes-plugin/hooks/
├── entrypoints/       # Hook entry points (SessionStart, PreToolUse, Stop, UserPromptSubmit)
├── lib/               # Shared utilities (git-state, state-manager, threshold calculation)
└── hooks.json         # Hook configuration and matchers
```

See [docs/bumper-lanes-threshold-flow.mmd](docs/bumper-lanes-threshold-flow.mmd) for detailed flow diagrams.

# Claude Bumper Lanes

**Purpose**: Enforce incremental code review by blocking Claude Code file modifications beyond configurable diff thresholds.

## Architecture Overview

Defense-in-depth hook system with three layers:

1. **Fuel Gauge** (PostToolUse): Escalating warnings after each Write/Edit - reaches Claude via stderr
2. **Enforcement** (Stop): Block Claude from finishing turn when threshold exceeded
3. **Manual Reset** (UserPromptSubmit): Intercept `/claude-bumper-lanes:bumper-reset` command to restore budget after review

## Technology Stack

- **Bash 4.0+**: Hook scripts and state management
- **Git 2.x+**: Working tree snapshots via `git write-tree`, diff calculation via `git diff-tree`
- **jq**: JSON parsing for hook I/O and session state
- **Claude Code Hooks**: SessionStart, PostToolUse, Stop, UserPromptSubmit events

## Design Principles

- **Stateful enforcement**: Track cumulative diff per session against baseline snapshot
- **Fail-open**: Errors allow operations (availability over strictness)
- **Explicit approval**: User must manually reset after reviewing changes
- **Transparent feedback**: Both user and Claude see threshold status and reasons

## Key Implementation Details

- Default threshold: 400 points (weighted scoring - edits 1.3× weight, new files 1.0×, deletions ignored)
- Session state persisted in `.git/bumper-checkpoints/session-{session_id}`
- Baseline reset captures current `git write-tree` SHA as new reference point
- PostToolUse fuel gauge tiers: 50% NOTICE, 75% WARNING, 90% CRITICAL
- Stop hook exit code 2 blocks Claude from finishing when threshold exceeded
- Scatter penalties: Extra points for touching many files (6-10: +10pts/file, 11+: +30pts/file)

## Project Structure

```
bumper-lanes-plugin/hooks/
├── entrypoints/       # Hook entry points (SessionStart, PostToolUse, Stop, UserPromptSubmit)
├── lib/               # Shared utilities (git-state, state-manager, threshold calculation)
└── hooks.json         # Hook configuration and matchers
```

See [docs/bumper-lanes-threshold-flow.mmd](docs/bumper-lanes-threshold-flow.mmd) for detailed flow diagrams.

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
- Session state persisted in `{git-dir}/bumper-checkpoints/session-{session_id}` (worktree-aware)
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

## Diff-Vizualization of diff scoring:

We're actively developing a catalogue of visualization tools to illustrate how different code changes usig git. This will help users understand how their modifications contribute to the overall score and encourage more incremental reviews.

### Diff-Viz Resources

@.agent-history/2025-12-22-diff-visualization-catalog.md
@.agent-history/2025-12-22-library-reference-map.md

### Adding New View Modes

When adding a new diff visualization mode, update ALL of these:

1. `tools/diff-viz/cmd/git-diff-tree/main.go` - mode flag and switch case
2. `hooks/lib/state-manager.sh` - `set_view_mode()` case validation + error message
3. `hooks/bin/set-view-mode.sh` - available modes help text
4. `commands/bumper-view.md` - argument-hint list
5. **Rebuild the binary**: `just build-diff-viz` (status line uses compiled binary, not `go run`)

## Just Commands

read the @justfile or use the just-mcp to run just commands instead of bash scripts directly.

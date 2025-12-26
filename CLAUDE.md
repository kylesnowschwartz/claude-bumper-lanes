# Claude Bumper Lanes

**Purpose**: Enforce incremental code review by blocking Claude Code file modifications beyond configurable diff thresholds.

## Architecture Overview

Defense-in-depth hook system with three layers:

1. **Fuel Gauge** (PostToolUse): Escalating warnings after each Write/Edit - reaches Claude via stderr
2. **Enforcement** (Stop): Block Claude from finishing turn when threshold exceeded
3. **Manual Reset** (UserPromptSubmit): Intercept `/claude-bumper-lanes:bumper-reset` command to restore budget after review

## Technology Stack

- **Go 1.21+**: Hook handler (`bumper-lanes`)
- **[diff-viz](https://github.com/kylesnowschwartz/diff-viz)**: External dependency for diff visualization (`git-diff-tree`)
- **Git 2.x+**: Working tree snapshots via `git write-tree`, diff calculation via `git diff-tree`
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
- PostToolUse fuel gauge tiers: 70% NOTICE, 90% WARNING
- Stop hook exit code 2 blocks Claude from finishing when threshold exceeded
- Scatter penalties: Extra points for touching many files (6-10: +10pts/file, 11+: +30pts/file)

## Hook-Intercept-Block Pattern

All slash commands use the "hook-intercept-block" pattern for instant execution without Claude API calls. This pattern intercepts user prompts via UserPromptSubmit hook, executes logic directly in Go, and returns `decision: "block"` with a `reason` message - bypassing the Claude API entirely.

**Key insight**: In Claude Code's hook API, `block` means "handled, don't call API" - not "rejected".

See the **hook-intercept-block** skill for full documentation on implementing new commands.

### Known Limitation: Statusline Refresh with Arguments

**Bug**: Claude Code only triggers immediate statusline refresh for blocked commands **without arguments**.

| Command | Statusline Refresh |
|---------|-------------------|
| `/bumper-pause` (no arg) | Immediate (~20ms) |
| `/bumper-view tree` (with arg) | Delayed until user interaction |

**Workaround**: Use separate no-arg commands for each option (e.g., `/bumper-tree`, `/bumper-icicle`) instead of a single command with arguments (`/bumper-view <mode>`).

**Tracked**: [anthropics/claude-code#15275](https://github.com/anthropics/claude-code/issues/15275)

## Configuration

Threshold is configurable via JSON files (priority order):

1. **Personal** (`.git/bumper-config.json`): Untracked, per-developer override
2. **Repo** (`.bumper-lanes.json`): Tracked, shared team default
3. **Default**: 400 points

### Config Commands

- `/bumper-config` - Show current configuration
- `/bumper-config 300` - Set repo threshold (creates `.bumper-lanes.json`)
- `/bumper-config personal 500` - Set personal threshold (in `.git/`, untracked)

### Config Schema

```json
{"threshold": 300}
```

Threshold range: 50-2000 points. After changing config, run `/bumper-reset` to apply to current session.

## Project Structure

```
bumper-lanes-plugin/
├── bin/               # Built binaries (bumper-lanes, git-diff-tree)
├── tools/
│   └── bumper-lanes/  # Hook handler and commands (Go)
├── commands/          # Slash command definitions
└── hooks.json         # Hook configuration and matchers
```

See [docs/bumper-lanes-threshold-flow.mmd](docs/bumper-lanes-threshold-flow.mmd) for detailed flow diagrams.

## Status Line Integration

The `bumper-lanes status` command supports modular widgets for integration with custom status lines (ccstatusline, bash scripts, etc.).

### Widget Modes

```bash
# Full output: status line + diff visualization (default)
bumper-lanes status --widget=all

# Just the threshold gauge: "active (125/400 - 31%)"
bumper-lanes status --widget=indicator

# Just the diff tree visualization
bumper-lanes status --widget=diff-tree
```

### Custom Status Line Example

```bash
#!/bin/bash
# Your custom status line script

# Get Claude Code's status JSON
claude_status_json=$(cat)

# Cherry-pick just the bumper-lanes indicator
bumper_gauge=$(echo "$claude_status_json" | bumper-lanes status --widget=indicator)

# Compose your own status line
echo "[$MY_CUSTOM_WIDGET] | $bumper_gauge | [other stuff]"
```

### Programmatic Access

The `statusline.StatusOutput` struct exposes components for Go integrations:

```go
type StatusOutput struct {
    StatusLine      string // Full line: model | dir | branch | cost | bumper
    BumperIndicator string // Just: "active (125/400 - 31%)"
    DiffTree        string // The visualization
    State           string // "active", "tripped", "paused", or ""
    Score, Limit, Percentage int
}
```

## Diff Visualization

Diff visualization is provided by [diff-viz](https://github.com/kylesnowschwartz/diff-viz), a standalone tool.

### Installation

```bash
# Install diff-viz
just install-diff-viz
# or: go install github.com/kylesnowschwartz/diff-viz/cmd/git-diff-tree@latest

# Bundle into plugin bin/ for distribution
just bundle-diff-viz
```

### Available Modes

Run `git-diff-tree --list-modes` for available visualization modes:
- `tree` - Indented file tree with +/- stats (default)
- `collapsed` - Single-line per directory
- `smart` - Depth-2 aggregated sparkline
- `topn` - Top 5 files by change size
- `icicle` - Horizontal area chart
- `brackets` - Nested `[dir file]` single-line

### Development

For diff-viz development, see https://github.com/kylesnowschwartz/diff-viz

## Just Commands

read the @justfile or use the just-mcp to run just commands instead of bash scripts directly.

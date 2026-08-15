# Bumper Lanes

Bumper lanes is a circuit breaker for Claude Code: it tracks how much code Claude has written since the last review and blocks Claude from continuing until the accumulated changes are reviewed — by you (the default), or by a code review the agent must run on itself before it may proceed.

![Threshold exceeded demo](assets/bumper-demo.gif)

## What It Does

Bumper-Lanes tracks how much code Claude has written or edited, blocking further edits when a threshold is exceeded. 600 points corresponds roughly to that many lines of code added, depending on the mix of new files vs edits.

When the threshold is exceeded:

1. **Fuel gauge warnings** show escalating alerts after each Write/Edit (70% NOTICE → 90% WARNING) to you and to Claude
2. **Stop hook** blocks Claude from continuing when threshold exceeded
3. **Reset command** (`/bumper-reset`) restores the budget after you review

Beyond the size budget, opt-in **tripwires** flag high-risk change classes (CI config, dependency manifests, disabled tests) immediately, at any score.

## Installation

```bash
claude plugin marketplace add kylesnowschwartz/claude-bumper-lanes
claude plugin install claude-bumper-lanes
```

**Requirements:** Go 1.21+ (binaries are built automatically on first session start)

## Usage

Work normally with Claude. If the configurable threshold is exceeded:

1. Claude will be blocked from continuing
2. Review your changes (`/bumper-diff` shows what changed since the baseline)
3. Optionally commit: `git add -u && git commit -m "message"` (resets the baseline automatically)
4. Or Manually reset the baseline: `/bumper-reset`
5. Continue working with restored budget

With `on_trip: review`, step 1 changes: instead of waiting for you, the trip instructs the agent to run a code review of the increment (your `review_command`), clear the breaker itself (`bumper-lanes review-clear`), and implement the findings against the fresh budget. Every self-clear is recorded in the event log, and `max_auto_reviews` caps how many happen before a trip comes back to you.

Pulling or rebasing mid-session is free: the baseline follows commits that land (under the default `reset_on: commit`), so upstream changes are never charged against the budget.

## Commands

| Command | Description |
|---------|-------------|
| `/bumper-diff` | Show the diff visualization (working tree vs review baseline) |
| `/bumper-reset` | Reset baseline after reviewing changes |
| `/bumper-pause` | Pause threshold enforcement (session only) |
| `/bumper-resume` | Resume threshold enforcement |
| `/bumper-config` | Show current configuration |
| `/bumper-config <n>` | Set repo threshold (0=disabled, 50-2000) |

Two CLI verbs run from the agent's shell (no slash command, no statusline JSON needed):

| Command | Description |
|---------|-------------|
| `bumper-lanes budget` | Print the remaining review budget in plain text (the agent uses this to size increments) |
| `bumper-lanes review-clear` | Clear a tripped breaker after a self-review (only valid with `on_trip: review`) |

## Status Line

The status line shows a one-line gauge: a traffic-light bar with the percentage of budget spent, a red ⚠ when a tripwire fired, and a green line count when the increment is net-negative (the tree shrank).

Status line setup is **opt-in** and only applies when you have no status line configured: answer yes to the "Add budget gauge to status line" prompt when enabling the plugin (or set `"statusline_auto_setup": true` in a repo `.bumper-lanes.json`) and the plugin points `~/.claude/settings.json` at the bumper-lanes binary on session start. An existing status line command is never touched — compose the gauge into it yourself with `bumper-lanes status --widget=indicator`. Or configure manually:

```json
{
  "statusLine": {
    "type": "command",
    "command": "/path/to/.claude/plugins/cache/.../bumper-lanes status",
    "padding": 0
  }
}
```

For custom status lines, `bumper-lanes status --widget=indicator` prints just the bumper gauge.

**Want rich diff visualizations?** Use `/bumper-diff` in a session, or install [diff-viz](https://github.com/kylesnowschwartz/diff-viz) globally for standalone use:

```bash
go install github.com/kylesnowschwartz/diff-viz/v2/cmd/git-diff-tree@latest
```

## Configuration

Set your defaults when enabling the plugin: `/plugin` > claude-bumper-lanes prompts for the threshold, reset policy, trip behavior, and status line setup, and stores them in your user settings. Change them any time through the same menu.

Precedence order:

1. `.bumper-lanes.json` at repo root (highest priority - per-repo overrides)
2. Plugin settings (set via `/plugin`, prompted at enable)
3. Built-in defaults

The pre-v5 global file (`~/.config/bumper-lanes/config.json`) is not read; if it exists, `/bumper-config` warns until you move its values and delete it.

```json
{
  "threshold": 400,
  "reset_on": "commit",
  "statusline_auto_setup": false
}
```

| Field | Description |
|-------|-------------|
| `threshold` | Points limit. `0` = disabled, `50-2000` = active (default: 600) |
| `reset_on` | When Claude's git commits auto-reset the budget: `commit` (default), `verified-commit` (refuses `--no-verify`), or `human` (never) |
| `on_trip` | What tripping asks for: `block` (default) shows the review packet and waits for you; `review` instructs the agent to run a code review of the increment, clear the breaker itself (`bumper-lanes review-clear`), and implement the findings against a fresh budget. |
| `max_auto_reviews` | Self-review clears allowed per human touchpoint: `1` (default), any `N`, `0` = never, `-1` = unlimited (hands-off: trips force a review but never require you) |
| `review_command` | The review workflow named in the self-review instruction (default `/code-review`) |
| `tripwires_block_auto_review` | When `true`, increments with tripwire hits cannot self-clear and always come to you (default `false`) |
| `statusline_auto_setup` | Allow session start to configure the status line (default: false) |
| `tripwire_paths` | Glob patterns whose changes are flagged at any score. Opt-in: omit to disable; the entry `"defaults"` expands to a recommended list (CI, dependency manifests, migrations) |
| `tripwire_patterns` | Added-line substrings that are flagged at any score. Opt-in: omit to disable; the entry `"defaults"` expands to a recommended list (test-skip idioms) |

### Disabling Enforcement

Set `"threshold": 0` in `.bumper-lanes.json` to disable for a specific repo, or set the plugin's threshold to `0` (`/plugin` > claude-bumper-lanes) to disable everywhere. Repo files override the plugin setting.

Run `/bumper-config` to see which config sources are active.

### Weighted Scoring

- **New file additions**: 1.0x weight
- **Edits to existing files**: 1.3x weight (harder to review)
- **Scatter penalty**: Extra points when touching many files
- **Deletions**: Not counted (removing code is good)
- **Generated files** (lockfiles, codegen, vendored): Not counted

## Requirements

- Go 1.21+ (for automatic binary compilation)
- Git 2.x+
- Claude Code with hooks support

## Project Structure

```
bumper-lanes-plugin/
├── bin/                    # Built binary (auto-generated)
│   └── bumper-lanes        # Hook handler
├── scripts/
│   └── ensure-binaries.sh  # Auto-builds on first run
├── tools/
│   └── bumper-lanes/       # Hook handler source (Go)
├── commands/               # Slash command definitions
└── hooks/
    └── hooks.json          # Hook configuration
```

Diff calculation and visualization are provided by [diff-viz](https://github.com/kylesnowschwartz/diff-viz), imported as a Go library.

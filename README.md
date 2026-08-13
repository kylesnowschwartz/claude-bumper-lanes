# Bumper Lanes

Bumper lanes is a circuit breaker for Claude Code: it tracks how much code Claude has written since your last review and blocks Claude from continuing until you review and approve the accumulated changes.

![Threshold exceeded demo](assets/bumper-demo.gif)

## What It Does

Bumper-Lanes tracks how much code Claude has written or edited, blocking further edits when a threshold is exceeded. 600 points corresponds roughly to that many lines of code added, depending on the mix of new files vs edits.

When the threshold is exceeded:

1. **Fuel gauge warnings** show escalating alerts after each Write/Edit (70% NOTICE → 90% WARNING) to you and to Claude
2. **Stop hook** blocks Claude from continuing when threshold exceeded
3. **Reset command** (`/bumper-reset`) restores the budget after you review

Beyond the size budget, **tripwires** flag high-risk change classes (CI config, dependency manifests, disabled tests) immediately, at any score.

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

## Status Line

The status line shows a one-line gauge: a traffic-light bar with the percentage of budget spent, a red ⚠ when a tripwire fired, and a green line count when the increment is net-negative (the tree shrank).

Status line setup is **opt-in**: set `"statusline_auto_setup": true` in your config and the plugin configures `~/.claude/settings.json` on session start. Or configure manually:

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

### Opting Out of Auto-Setup

To prevent bumper-lanes from modifying your statusline script, add this comment anywhere in your script:

```bash
# BUMPER_HANDS_OFF
```

This tells bumper-lanes to leave your configuration alone. The plugin will not wrap, update, or regenerate any script containing this marker.

## Configuration

Config files (in precedence order):
1. `.bumper-lanes.json` at repo root (highest priority)
2. `~/.config/bumper-lanes/config.json` (global fallback)
3. Built-in defaults

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
| `tripwire_paths` | Glob patterns whose changes are flagged at any score (defaults cover CI, dependency manifests, migrations) |
| `tripwire_patterns` | Added-line substrings that are flagged at any score (defaults cover test-skip idioms) |

### Disabling Enforcement

Set `"threshold": 0` in `.bumper-lanes.json` to disable for a specific repo, or in the global config to disable everywhere. Individual repos can override the global config.

Run `/bumper-config` to see which config files are active.

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

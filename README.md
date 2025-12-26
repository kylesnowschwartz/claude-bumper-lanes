![Bumper Lanes](bumper-lanes.png)

Vibe coding too much? Losing discipline and track of your changes? Add bumper lanes to your Claude Code sessions, and stay out of the gutters!

## What It Does

Bumper-Lanes tracks how much code Claude has written or edited, blocking further edits when a threshold is exceeded. 400 points corresponds roughly to 300-500 lines of code changed, depending on the mix of new files vs edits.

When the threshold is exceeded:

1. **Fuel gauge warnings** show escalating alerts after each Write/Edit (50% → 75% → 90%)
2. **Stop hook** blocks Claude from finishing when threshold exceeded
3. **Reset command** (`/bumper-reset`) restores the budget after you review

### Weighted Scoring

- **New file additions**: 1.0x weight
- **Edits to existing files**: 1.3x weight (harder to review)
- **Scatter penalty**: Extra points when touching many files
- **Deletions**: Not counted (removing code is good)

## Installation

```bash
claude plugin marketplace add kylesnowschwartz/claude-bumper-lanes
claude plugin install claude-bumper-lanes
```

**Requirements:** Go 1.21+ (binaries are built automatically on first session start)

## Usage

Work normally with Claude. When the threshold is exceeded:

1. Claude will be blocked from continuing
2. Review changes: `git diff` or `git status`
3. Optionally commit: `git add -u && git commit -m "message"`
4. Reset baseline: `/bumper-reset`
5. Continue working with restored budget

## Commands

| Command | Description |
|---------|-------------|
| `/bumper-reset` | Reset baseline after reviewing changes |
| `/bumper-pause` | Pause threshold enforcement |
| `/bumper-resume` | Resume threshold enforcement |
| `/bumper-config` | Show current configuration |
| `/bumper-config set <n>` | Set repo threshold (50-2000) |
| `/bumper-config personal <n>` | Set personal threshold |

### View Modes

| Command | Description |
|---------|-------------|
| `/bumper-tree` | Tree view with file hierarchy |
| `/bumper-collapsed` | Single-line grouped by directory |
| `/bumper-icicle` | Flame chart showing hierarchy by width |
| `/bumper-topn` | Top N files by change size |
| `/bumper-pathstrip` | Abbreviated paths |

## Status Line Setup

Status line is **auto-configured** on first session. No manual setup needed.

If you need to configure manually:

**Option 1: Use the binary directly** (no existing status line)

Add to `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "/path/to/bumper-lanes-plugin/bin/bumper-lanes"
  }
}
```

**Option 2: Add to existing status line**

Claude Code status lines can be multi-line: line 1 is the compact status bar, additional lines are widgets.

Add to the **end** of your status line script (after your main output):

```bash
# Bumper-lanes widgets (requires input=$(cat) at script start)
bumper_indicator=$(echo "$input" | bumper-lanes status --widget=indicator)
diff_tree=$(echo "$input" | bumper-lanes status --widget=diff-tree)
[[ -n "$bumper_indicator" ]] && echo "$bumper_indicator"
[[ -n "$diff_tree" ]] && echo "$diff_tree"
```

Run `/bumper-setup-statusline` for setup instructions.

### Opting Out of Auto-Setup

To prevent bumper-lanes from modifying your statusline script, add this comment anywhere in your script:

```bash
# BUMPER_HANDS_OFF
```

This tells bumper-lanes to leave your configuration alone. The plugin will not wrap, update, or regenerate any script containing this marker.

## Configuration

Config file: `.bumper-lanes.json` at repo root. Add to `.gitignore` if you don't want to track it.

```json
{
  "threshold": 400,
  "default_view_mode": "tree",
  "default_view_opts": "--width 80 --depth 3"
}
```

## Requirements

- Go 1.21+ (for automatic binary compilation)
- Git 2.x+
- Claude Code with hooks support

## Project Structure

```
bumper-lanes-plugin/
├── bin/                    # Built binaries (auto-generated)
│   ├── bumper-lanes        # Hook handler
│   └── git-diff-tree       # Diff visualization (from diff-viz)
├── scripts/
│   └── ensure-binaries.sh  # Auto-builds on first run
├── tools/
│   └── bumper-lanes/       # Hook handler source (Go)
├── commands/               # Slash command definitions
└── hooks/
    └── hooks.json          # Hook configuration
```

Diff visualization is provided by [diff-viz](https://github.com/kylesnowschwartz/diff-viz).

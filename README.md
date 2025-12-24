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

## Configuration

| File | Tracked | Purpose |
|------|---------|---------|
| `.git/bumper-config.json` | No | Personal overrides |
| `.bumper-lanes.json` | Yes | Shared team defaults |

```json
{
  "threshold": 400,
  "default_view_mode": "tree"
}
```

## Requirements

- Git 2.x+
- Claude Code with hooks support

## Project Structure

```
bumper-lanes-plugin/
├── bin/                    # Built binaries
│   ├── bumper-lanes        # Hook handler (Go)
│   └── git-diff-tree-go    # Diff visualization (Go)
├── tools/
│   ├── bumper-lanes/       # Hook handler source
│   └── diff-viz/           # Diff visualization source
├── commands/               # Slash command definitions
└── hooks.json              # Hook configuration
```

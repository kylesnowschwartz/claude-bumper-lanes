![Bumper Lanes](bumper-lanes.png)

Vibe coding too much? Losing discipline and track of your changes? Add bumper lanes to your Claude Code sessions, and stay out of the gutters!

## What It Does

Put simply, Bumper-Lanes keeps track of how much code Claude has written or edited, and prevents further edits when a
threshold of changes has been recahed. 400 points corresponds roughly to 300-500 lines of code changed, depending on the
mix of new files vs edits.

Bumper-Lanes generates its weighted diff threshold with working tree snapshots  via `git write-tree`, and the diff calculation is performed
via `git diff-tree`. When the threshold is exceeded:

1. **PostToolUse hook** shows escalating fuel gauge warnings after each Write/Edit
2. **Stop hook** blocks Claude from finishing when threshold exceeded
3. **Reset command** (`/bumper-reset`) restores the budget after review

### Weighted Scoring

Uses weighted scoring instead of simple line counts to better reflect review difficulty:

- **New file additions**: 1.0× weight
- **Edits to existing files**: 1.3× weight (harder to review)
- **Scatter penalty**: Extra points when touching many files
- **Deletions**: Not counted (removing code is good)

## Installation

```bash
claude plugin marketplace add kylesnowschwartz/claude-bumper-lanes
claude plugin install claude-bumper-lanes

# Update
claude plugin marketplace update claude-bumper-lanes
```

## Usage

Work normally with Claude. When the threshold is exceeded:

1. Claude will inform you that changes exceed the limit
2. Review changes: `git diff` or `git status` etc.
3. Optionally commit: `git add -u && git commit -m "message"`
4. Reset baseline: `/claude-bumper-lanes:bumper-reset`
5. Continue working with restored budget

## Status Line

Bumper-Lanes can't add a status line for your, but it provides an example status line you can use to modify your own or
use directly.

**Display format**: `bumper-lanes active (145/400 · 36%)` or `bumper-lanes tripped (425/400 · 106%)`

- **Green** when active (under threshold)
- **Red** when tripped (exceeded threshold)
- Shows both absolute points and percentage in real-time

Requires `jq` and `awk`. Your script must read status line JSON into `$input`. See `status-lines/simple-status-line.sh` for full example.

## How It Works

- Fuel gauge warnings (PostToolUse) + enforcement blocking (Stop hook)
- Escalating alerts: 50% NOTICE → 75% WARNING → 90% CRITICAL
- Tracks cumulative diff per session using Git snapshots
- Explicit user manual reset required to continue

See [docs/bumper-lanes-threshold-flow.mmd](docs/bumper-lanes-threshold-flow.mmd) for architecture details.

## Project Structure

```
bumper-lanes-plugin/
├── commands/
│   └── bumper-reset.md       # Slash command metadata
└── hooks/
    ├── bin/                      # Utility scripts
    │   ├── pause-baseline.sh     # Pause threshold enforcement
    │   ├── reset-baseline.sh     # Reset diff baseline
    │   └── resume-baseline.sh    # Resume threshold enforcement
    ├── entrypoints/              # Hook entry points
    │   ├── post-tool-use-feedback.sh # Fuel gauge warnings
    │   ├── stop.sh               # Block Claude when threshold exceeded
    │   ├── user-prompt-submit.sh # Intercept /bumper-reset command
    │   └── session-start.sh      # Initialize session state
    ├── lib/                      # Shared utilities
    │   ├── git-state.sh          # Git tree snapshots
    │   ├── state-manager.sh      # Session state persistence
    │   └── threshold-calculator.sh # Weighted threshold calculation
    └── hooks.json                # Hook configuration
```

## Requirements

- Bash 4.0+
- Git 2.x+
- jq (JSON parsing)
- Claude Code with hooks support

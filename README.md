```
    ____                                     __
   / __ )__  ______ ___  ____  ___  _____   / /   ____ _____  ___  _____
  / __  / / / / __ `__ \/ __ \/ _ \/ ___/  / /   / __ `/ __ \/ _ \/ ___/
 / /_/ / /_/ / / / / / / /_/ /  __/ /     / /___/ /_/ / / / /  __(__  )
/_____/\__,_/_/ /_/ /_/ .___/\___/_/     /_____/\__,_/_/ /_/\___/____/
                     /_/
```

Vibe coding too much? Losing discipline and track of your changes? Add bumper lanes to your Claude Code sessions, and stay out of the gutters!

Enforce incremental code review gates in Claude Code by blocking unbounded changes.

## What It Does

Prevents Claude from making changes beyond a configurable line-diff threshold (default: 200 lines). When the threshold is exceeded:

1. **PreToolUse hook** blocks `Write` and `Edit` tools before they execute
2. **Stop hook** prevents Claude from finishing until you review
3. **Reset command** (`/bumper-reset`) restores the budget after review

## Installation

```bash
claude plugin marketplace add kylesnowschwartz/claude-bumper-lanes
claude plugin install claude-bumper-lanes
```

## Usage

Work normally with Claude. When the threshold is exceeded:

1. Claude will inform you that changes exceed the limit
2. Review changes: `git diff` or `git status` etc.
3. Optionally commit: `git add -u && git commit -m "message"`
4. Reset baseline: `/claude-bumper-lanes:bumper-reset`
5. Continue working with restored budget

## Configuration

Default threshold: **200 lines changed** (additions + deletions)

## How It Works

- Proactive blocking (PreToolUse hook) + reactive notification (Stop hook)
- Tracks cumulative diff per session using Git snapshots
- Explicit user manual reset

See [docs/bumper-lanes-threshold-flow.mmd](docs/bumper-lanes-threshold-flow.mmd) for architecture details.

## Project Structure

```
bumper-lanes-plugin/hooks/
├── commands/
│   └── reset-baseline.sh # Manual reset command
├── entrypoints/          # Hook entry points
│   ├── pre-tool-use.sh   # Block Write/Edit tools
│   ├── stop.sh           # Block Claude stop
│   ├── user-prompt-submit.sh  # Intercept /bumper-reset
│   ├── session-start.sh  # Initialize session state
│   └── reset-baseline.sh # Reset diff baseline
├── lib/                  # Shared utilities
│   ├── git-state.sh      # Git tree snapshots
│   ├── state-manager.sh  # Session state persistence
│   └── threshold.sh      # Threshold calculation
└── hooks.json            # Hook configuration
```

## Requirements

- Bash 4.0+
- Git 2.x+
- jq (JSON parsing)
- Claude Code with hooks support

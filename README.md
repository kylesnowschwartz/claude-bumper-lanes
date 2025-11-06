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

Prevents Claude from making changes beyond a configurable weighted diff threshold (default: 400 points). When the threshold is exceeded:

1. **PreToolUse hook** blocks `Write` and `Edit` tools before they execute
2. **Stop hook** prevents Claude from finishing until you review
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

## Configuration

Default threshold: **400 points** (weighted score, not simple line count)

## Status Line Widget

Add bumper lanes status to your custom status line by copying these functions:

```bash
get_bumper_lanes_status() {
  local session_id=$(echo "$input" | jq -r '.session_id')
  local checkpoint_file=".git/bumper-checkpoints/session-$session_id"
  [[ ! -f "$checkpoint_file" ]] && return
  local stop_triggered=$(jq -r '.stop_triggered // false' "$checkpoint_file" 2>/dev/null)
  [[ "$stop_triggered" == "true" ]] && echo "tripped" || echo "active"
}

BUMPER_STATUS=$(get_bumper_lanes_status)
if [[ -n "$BUMPER_STATUS" ]]; then
  if [[ "$BUMPER_STATUS" == "active" ]]; then
    output+=" | $(printf "\e[32mbumper-lanes active\e[0m")"
  else
    output+=" | $(printf "\e[31mbumper-lanes tripped\e[0m")"
  fi
fi
```

Requires `jq` and that your script reads status line JSON into `$input`. See `status-lines/simple-status-line.sh` for full example.

## How It Works

- Proactive blocking (PreToolUse hook) + reactive notification (Stop hook)
- Tracks cumulative diff per session using Git snapshots
- Explicit user manual reset

See [docs/bumper-lanes-threshold-flow.mmd](docs/bumper-lanes-threshold-flow.mmd) for architecture details.

## Project Structure

```
bumper-lanes-plugin/
├── commands/
│   └── bumper-reset.md       # Slash command metadata
└── hooks/
    ├── entrypoints/              # Hook entry points
    │   ├── pre-tool-use.sh       # Block Write/Edit tools
    │   ├── stop.sh               # Block Claude stop
    │   ├── user-prompt-submit.sh # Intercept /bumper-reset command
    │   ├── session-start.sh      # Initialize session state
    │   └── reset-baseline.sh     # Reset diff baseline
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

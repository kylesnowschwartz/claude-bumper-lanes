# Statusline Data Flow

## How Claude Code Calls the Statusline

Claude Code's status line setting points to `bumper-lanes status`, which receives JSON on stdin and outputs the formatted status line.

```
┌─────────────┐         stdin (JSON)         ┌──────────────────┐
│ Claude Code │ ──────────────────────────▶  │ bumper-lanes     │
│             │                              │ status           │
│             │  ◀──────────────────────────  │                  │
└─────────────┘         stdout (text)        └──────────────────┘
```

## Statusline Render: What Gets Called

The render path reads only cached session state plus two cheap git reads for the branch display. It never computes a baseline diff: a working-tree capture costs ~60-110ms, too slow to pay per refresh.

```
bumper-lanes status
    │
    ├─▶ git branch --show-current       # branch display (~10ms)
    ├─▶ git diff --quiet HEAD           # dirty flag (~10ms)
    │
    ├─▶ state.Load(sessionID)           # Read session JSON from disk (~20ms)
    │       │
    │       └─▶ Returns: Score, NetLines, Tripwires, Paused, StopTriggered
    │
    └─▶ formatBumperStatus(...)         # "▂ 31%" + optional ⚠ + green net lines
```

## When Score (and NetLines) Get Updated

Score and net lines are recalculated from the baseline diff and cached in session state at these points:

1. **PostToolUse (Write/Edit/MultiEdit/NotebookEdit)** — fresh `getStatsJSON(baseline)` → `scoring.Calculate` → `sess.SetScore` + `sess.NetLines`, saved to disk.
2. **Stop hook** — same fresh recalculation on every turn end (enables auto-recovery when the score drops).
3. **PreToolUse (after a trip)** — recalculates to detect external commits and auto-recovery.
4. **Baseline resets** (`/bumper-reset`, commit auto-reset, branch switch, clean tree) — score and net lines zeroed with the new baseline.

## Accepted Staleness

Between hook events (e.g. the user edits a file in another editor), the gauge is stale until the next Write/Edit or Stop. This is deliberate: the statusline is an ambient indicator, and `/bumper-diff` gives an on-demand, always-fresh view of the working tree vs the review baseline.

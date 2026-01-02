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

```
bumper-lanes status
    │
    ├─▶ state.Load(sessionID)           # Read session JSON from disk (~20ms)
    │       │
    │       └─▶ Returns: sess.Score, sess.BaselineTree, etc.
    │
    ├─▶ score = sess.Score              # CACHED - no git commands
    │
    ├─▶ formatBumperStatus(...)         # Format "active (125/400 - 31%)"
    │
    └─▶ getDiffTree(viewMode, viewOpts) # FRESH - runs git commands (~70ms)
            │
            ├─▶ diff.GetAllStats()      # git diff --numstat HEAD
            │                           # git ls-files --others
            │
            └─▶ render.Render(stats)    # Format tree visualization
```

## When Score Gets Updated

### 1. Session Start
```
SessionStart hook
    │
    └─▶ CaptureTree()                   # git write-tree (~300ms)
            │
            └─▶ sess.BaselineTree = tree
                sess.Score = 0          # Initial score
                sess.Save()
```

### 2. After Write/Edit (PostToolUse)
```
Claude calls Write/Edit tool
    │
    └─▶ PostToolUse hook fires
            │
            ├─▶ state.Load(sessionID)
            │
            ├─▶ getStatsJSON(baseline)   # git diff-tree baseline..current (~300ms)
            │       │
            │       ├─▶ CaptureTree()    # git write-tree
            │       └─▶ GetTreeDiffStats()
            │
            ├─▶ scoring.Calculate(stats)
            │
            ├─▶ sess.SetScore(freshScore)
            │
            └─▶ sess.Save()              # Score now cached on disk
```

### 3. After /bumper-reset
```
User types /bumper-reset
    │
    └─▶ UserPromptSubmit hook intercepts
            │
            ├─▶ state.Load(sessionID)
            │
            ├─▶ CaptureTree()            # git write-tree (~300ms)
            │
            ├─▶ sess.ResetBaseline(tree)
            │       │
            │       └─▶ sess.Score = 0   # Reset score
            │
            ├─▶ sess.Save()
            │
            └─▶ blockPrompt("Baseline reset. Score: 0/600")
                    │
                    └─▶ Claude Code renders statusline (new render)
```

### 4. After /bumper-tree (View Mode Change)
```
User types /bumper-tree
    │
    └─▶ UserPromptSubmit hook intercepts
            │
            ├─▶ state.Load(sessionID)         # ~20ms
            │
            ├─▶ sess.SetViewMode("tree")      # Instant
            │
            ├─▶ sess.Save()                   # ~5ms
            │
            └─▶ blockPrompt("View: tree")     # Returns to Claude Code
                    │
                    └─▶ Claude Code renders statusline
                            │
                            └─▶ Statusline sees new ViewMode
                                Diff-viz renders with "tree" mode
```
**Total time before block response: ~25ms**

### 5. After git commit (PostToolUse Bash)
```
Claude runs: git commit -m "message"
    │
    └─▶ PostToolUse hook (Bash) fires
            │
            ├─▶ Detects git commit pattern
            │
            ├─▶ git rev-parse HEAD^{tree}
            │
            ├─▶ sess.ResetBaseline(tree)  # Score = 0
            │
            └─▶ sess.Save()
```

## Timing Comparison: /bumper-reset vs /bumper-tree

```
/bumper-reset                              /bumper-tree
─────────────                              ────────────
T=0ms   UserPromptSubmit intercepts        T=0ms   UserPromptSubmit intercepts
T=20ms  state.Load()                       T=20ms  state.Load()
T=320ms CaptureTree() ◄── SLOW             T=20ms  SetViewMode() ◄── INSTANT
T=325ms ResetBaseline()                    T=25ms  Save()
T=340ms Save()                             T=25ms  blockPrompt() returns
T=340ms blockPrompt() returns
        │                                          │
        ▼                                          ▼
T=340ms Claude Code receives block         T=25ms  Claude Code receives block
T=???   Claude Code triggers statusline    T=???   Claude Code triggers statusline
        refresh (timing unknown)                   refresh (timing unknown)
```

**Key difference**: `/bumper-reset` takes ~340ms before returning the block response.
`/bumper-tree` returns in ~25ms.

### What We Don't Know (Claude Code internals)

1. **When does Claude Code refresh the statusline after a blocked command?**
   - Immediately after receiving block response?
   - On next 300ms poll cycle?
   - Some other trigger?

2. **Does the 340ms delay in /bumper-reset affect the refresh?**
   - Maybe Claude Code has a timeout for hook responses?
   - Maybe slow hooks get deprioritized for refresh?

3. **Is there a race condition?**
   - If statusline polls during /bumper-reset's CaptureTree()
   - The poll might read stale sess.Score before Save() completes

## The Problem: Score vs Diff-Viz Freshness

| Widget | Data Source | When Updated | Freshness |
|--------|-------------|--------------|-----------|
| **Bumper Status** | `sess.Score` (cached) | PostToolUse, /reset, commit | STALE between updates |
| **Diff-Viz Tree** | `diff.GetAllStats()` (live) | Every render | ALWAYS FRESH |

### Timeline Example

```
T=0    User makes change outside Claude (manual edit in VSCode)
       - sess.Score = 0 (stale, doesn't know about change)
       - diff.GetAllStats() = shows the change (fresh)

T=1    Statusline renders
       - Bumper status shows: "active (0/600 - 0%)"    ← WRONG
       - Diff-viz shows: the actual file change        ← CORRECT

T=2    Claude runs Write tool
       - PostToolUse fires
       - sess.Score recalculated = 50
       - sess.Save()

T=3    Statusline renders
       - Bumper status shows: "active (50/600 - 8%)"   ← NOW CORRECT
       - Diff-viz shows: current state                 ← CORRECT
```

## Options to Fix Score Staleness

### Option A: Always Recalculate (No Cache)
```go
// In statusline render:
score = calculateScore(sess.BaselineTree)  // Fresh every time (~300ms)
```
- **Pro**: Always accurate
- **Con**: 300ms per render, ~6 git process spawns

### Option B: Cache with Tree SHA Validation
```go
// In session state:
type SessionState struct {
    Score         int
    ScoreTreeSHA  string  // Tree SHA when score was calculated
}

// In statusline render:
currentTree := quickGetTreeSHA()  // Fast check
if currentTree == sess.ScoreTreeSHA {
    score = sess.Score  // Cache hit
} else {
    score = calculateScore(sess.BaselineTree)
    sess.ScoreTreeSHA = currentTree
    sess.Save()
}
```
- **Pro**: Fresh when needed, fast when unchanged
- **Con**: Still need fast tree SHA check

### Option C: Hybrid - Use Diff-Viz Stats for Score
```go
// In statusline render:
stats, _, _ := diff.GetAllStats()  // Already called for diff-viz
score = scoring.CalculateFromStats(stats, sess.BaselineTree)
```
- **Pro**: Reuses existing git calls
- **Con**: GetAllStats is HEAD-relative, not baseline-relative

### Option D: Accept Staleness
- Score updates on Write/Edit/Reset/Commit
- Between those events, score may be stale
- Diff-viz is always fresh as "ground truth"

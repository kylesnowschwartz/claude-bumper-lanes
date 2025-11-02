# Research: Bumper Lanes

**Feature**: Bumper Lanes Plugin
**Date**: 2025-11-02
**Status**: Complete

## Overview

This document captures research findings for the Bumper Lanes Claude Code plugin, which enforces diff thresholds to promote disciplined code review during AI agent sessions.

## Key Research Questions

### Q1: How can we track working tree state across multiple checkpoints without creating git commits?

**Decision**: Use git tree objects via temporary index file approach

**Rationale**:
- Git's plumbing commands (`git write-tree`, `GIT_INDEX_FILE`) allow capturing working tree state as persistent tree objects
- Tree objects survive git operations (checkout, merge, rebase) once stored in `.git/objects/`
- Non-destructive: Using temporary index preserves actual staging area state
- Handles both tracked and untracked files when combined with `git ls-files --others --exclude-standard`

**Alternatives Considered**:
1. **Git stash**: Creates referenceable commits but pollutes stash history, complex cleanup
2. **Patch files**: `.diff` or `.patch` files are text-based but harder to diff against current state
3. **File hashing**: Would require tracking every file individually, complex for nested directories
4. **Commit-based**: Violates requirement of not creating commits

**Evidence**:
- Stack Overflow: "Create a git tree from working tree without touching the index" (2016)
- Git Documentation: git-write-tree, git-diff-tree
- Validated via prototype testing (see evidence-log.csv)

**Implementation**:
```bash
# Capture working tree as tree object (non-destructive)
TMP_INDEX=$(mktemp)
export GIT_INDEX_FILE=$TMP_INDEX
git add -u .
git ls-files --others --exclude-standard | xargs -r git add
TREE=$(git write-tree)
unset GIT_INDEX_FILE
rm -f $TMP_INDEX
echo $TREE  # Returns SHA-1 like "3a4b5c6d..."
```

### Q2: How do we compute diff statistics between two arbitrary working tree states?

**Decision**: Use `git diff-tree` with `--numstat` and `--shortstat` flags

**Rationale**:
- `git diff-tree` compares two tree objects directly without requiring commits
- `--numstat`: Provides per-file line counts (added, deleted) in machine-readable format
- `--shortstat`: Provides aggregate summary (files changed, insertions, deletions)
- Handles binary files (reports as `-` in numstat output)

**Implementation**:
```bash
# Compare two tree objects
git diff-tree --shortstat $BASELINE_TREE $CURRENT_TREE
# Output: 5 files changed, 23 insertions(+), 8 deletions(-)

# Per-file statistics
git diff-tree --numstat $BASELINE_TREE $CURRENT_TREE
# Output:
# 3       1       src/main.rs
# 12      7       tests/unit.rs
```

**Evidence**: Git Documentation - git-diff-tree (2024)

### Q3: How do Claude Code hooks work and what response formats do they support?

**Decision**: Use SessionStart, Stop/SubagentStop, and UserPromptSubmit hooks with JSON responses

**Rationale**:
- **SessionStart**: Captures baseline tree on session initialization, writes to `CLAUDE_ENV_FILE`
- **Stop/SubagentStop**: Checks diff stats on agent completion, returns `{"decision": "block"}` to prevent stopping
- **UserPromptSubmit**: Detects `/bumper-reset` command, resets baseline

**Hook Response Format**:
```json
{
  "decision": "block",
  "reason": "Diff threshold exceeded: 450/300 lines changed. Review code and run /bumper-reset to continue."
}
```

**Evidence**: Claude Code Documentation - Hooks (docs.claude.com, 2024)

### Q4: How do we isolate state between concurrent Claude sessions in the same repository?

**Decision**: PID-based state files in `.git/bumper-checkpoints/`

**Rationale**:
- Each hook execution has unique `$$` (process ID)
- State files named with PID: `baseline-12345`, `stats-12345`
- Cleanup on SessionEnd hook
- Works universally (not dependent on git worktrees)

**Alternatives Considered**:
1. **Worktree-aware**: Would only work for users using worktrees, not universal
2. **Shared state with locking**: Complex, race conditions, single point of failure
3. **Session ID from Claude**: Not exposed in hook stdin

**Implementation**:
```bash
# Store baseline with PID
echo "$BASELINE_TREE" > .git/bumper-checkpoints/baseline-$$

# Later retrieval
if [[ -f .git/bumper-checkpoints/baseline-$$ ]]; then
  BASELINE=$(cat .git/bumper-checkpoints/baseline-$$)
fi
```

### Q5: What threshold metric should we use for MVP?

**Decision**: Simple total line count (additions + deletions)

**Rationale**:
- Easy to understand and explain to users
- Matches common mental model of "how much changed"
- Computationally trivial from `git diff-tree --shortstat`
- Default: 300 lines total

**Alternatives Considered**:
1. **Weighted metric**: Adds=1, Deletes=0.5, Binary=10 (deferred to v2)
2. **File count**: Less granular, doesn't reflect size of changes
3. **Complexity-based**: Cyclomatic complexity (too slow for hook execution)

**Configuration Structure** (ready for future metrics):
```json
{
  "threshold": {
    "metric": "simple-line-count",
    "limit": 300,
    "weights": {
      "additions": 1,
      "deletions": 1,
      "binary_files": 1
    }
  }
}
```

### Q6: What consent mechanism should we use?

**Decision**: `/bumper-reset` slash command

**Rationale**:
- Explicit and intentional action
- Idempotent (can run multiple times safely)
- Discoverable (appears in slash command list)
- Follows Claude Code conventions

**Alternatives Considered**:
1. **Fuzzy phrase matching**: "LGTM", "continue", "looks good" (error-prone)
2. **Interactive prompt**: Blocks workflow, requires user attention immediately
3. **Auto-reset on commit**: Ties plugin behavior to commit workflow (not universal)

**Implementation**: UserPromptSubmit hook detects command and resets baseline

## Technical Constraints

1. **Performance**: Hook execution must complete in <500ms to avoid workflow disruption
2. **Non-destructive**: Must preserve git index state (staged vs unstaged distinction)
3. **Reliability**: Must handle git errors gracefully (non-git repos, missing baseline, corrupted state)
4. **Compatibility**: Bash 4.0+, Git 2.x+, macOS/Linux

## Edge Cases Identified

1. **Non-git repository**: Plugin should detect and disable gracefully
2. **Missing baseline**: First hook invocation should establish baseline
3. **Baseline commit deleted**: State file approach prevents this (tree objects persist)
4. **Binary files**: Counted as file changes but no line stats
5. **Submodules**: Detected and handled separately (commit SHA instead of tree)
6. **Large repos**: May need optimization for >100k files (deferred to profiling phase)
7. **Concurrent sessions**: Handled via PID isolation
8. **Manual edits vs agent edits**: Plugin tracks all changes (no distinction at git level)

## Open Questions & Risks

### Q: How do we handle SessionEnd cleanup if process crashes?

**Risk**: Orphaned state files in `.git/bumper-checkpoints/`

**Mitigation**:
- State files are small (<100 bytes each)
- Add `/bumper-cleanup` command to manually remove orphaned states
- Consider age-based cleanup (remove files older than 24h)

### Q: What happens if user switches branches mid-session?

**Risk**: Baseline tree may reference objects not reachable from new branch

**Mitigation**:
- Tree objects persist in `.git/objects/` regardless of branch
- `git diff-tree` will still work as long as objects exist
- Document this as expected behavior

### Q: Should we block subagent stops or only main agent stops?

**Decision Needed**: Current spec blocks both Stop and SubagentStop

**Consideration**: Subagents are often read-only or make minimal changes. Should they share the same threshold?

**Recommendation**: Block both by default, add configuration to exclude subagents if needed

## Sources

See `research/source-register.csv` and `research/evidence-log.csv` for complete audit trail.

## Next Steps

1. Phase 1: Create data-model.md defining state entities
2. Phase 1: Design hook contracts (stdin/stdout schemas)
3. Phase 1: Write quickstart.md for developer onboarding
4. Phase 2: Generate tasks.md breaking down implementation

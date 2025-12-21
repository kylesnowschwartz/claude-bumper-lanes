---
name: test-threshold
description: Stress test bumper lanes by creating files until threshold blocks
argument-hint: "[optional: target points, default 500]"
---

# Test Bumper Lanes Threshold

Create temporary files incrementally to test threshold enforcement and fuel gauge behavior.

## Workflow

### Step 1: Check Current State

```bash
# Get current diff stats
git diff --stat
```

Report current points if any accumulated.

### Step 2: Create Test Files

Create temporary test files in batches until blocked or target reached.

Target: $ARGUMENTS points (default: 500 if not specified)

Each file should be ~40-50 lines to make math easy (1 line ≈ 1 point for new files).

Create files named `tmp-threshold-test-{n}.txt` where n increments.

### Step 3: Observe Behavior

After each batch, note:
- Did PreToolUse block? (should block before execution)
- Did fuel gauge message appear?
- What was the reported score?

### Step 4: Report Results

When blocked or target reached, report:

| Metric | Value |
|--------|-------|
| Files created | X |
| Estimated lines | Y |
| Final score | Z pts (N%) |
| Blocked by | PreToolUse / Stop / Neither |

### Step 5: Cleanup Recommendation

Suggest running:
```bash
rm tmp-threshold-test-*.txt
```

Or offer to delete the files.

## Success Criteria

- [ ] Created enough content to exceed threshold
- [ ] Observed which hook blocked (PreToolUse or Stop)
- [ ] Fuel gauge messages appeared (or noted absence)
- [ ] Reported clear test results

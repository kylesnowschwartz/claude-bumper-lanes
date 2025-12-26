---
name: test-threshold
description: Stress test bumper lanes by creating files until threshold blocks
argument-hint: "[optional: target points, default 500]"
model: haiku
---

# Test Bumper Lanes Threshold

Create temporary files incrementally to test threshold enforcement and fuel gauge behavior.

## Workflow

### Step 1: Create Test Files

Create temporary test files in batches until blocked or target reached.

Target: $ARGUMENTS points (default: 500 if not specified)

Each file should be 75 lines to make math easy (1 line ≈ 1 point for new files).

Create files named `tmp-threshold-test-{n}.txt` where n increments.

### Step 2: Observe Behavior

After each batch, note:
- Did PreToolUse block? (should block before execution)
- Did fuel gauge message appear?
- What was the reported score?

### Step 3: Cleanup Recommendation

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

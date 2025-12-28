---
description: Generate nested test files to QA diff visualization modes (tree, sparkline, etc.)
argument-hint: [optional: file count, default 8] [optional: in squence or parallel]
model: haiku
---

# Test git-diff-tree Visualizations

Generate a nested directory structure with test files to validate all diff visualization modes (tree, collapsed, smart, topn).

## Workflow

### Step 1: Create Test Directory Structure

Create nested directories that will produce an interesting tree:

```
diff-viz/
cmd/
internal/
tmp-diff-tree-test/
├── src/
│   ├── components/
│   │   ├── Button.tsx
│   │   └── Modal.tsx
│   ├── utils/
│   │   └── helpers.ts
│   └── index.ts
├── tests/
│   └── unit/
│       └── button.test.ts
└── README.md
```

Target: $ARGUMENTS files (default: 8 if not specified)

### Step 2: Generate Files all at once, or if the user specifies, sequentially

Each file should have different line counts to test stat rendering:
- Small files: 5-10 lines
- Medium files: 20-40 lines
- Large files: 80-120 lines

The content lorem ipsum or some very well-trodden code

### Step 3: Report Results

For each mode tested, verify the results of `git-diff-tree --demo`:

### Step 5: Cleanup

Offer to remove test files when the user is ready

## Success Criteria

- [ ] Sequential file creation shows diff growing incrementally
- [ ] Each visualization mode produces distinct, readable output
- [ ] Nested directories render correctly in tree mode (no orphaned `│` pipes)
- [ ] Sparkline modes scale appropriately to change magnitude
- [ ] Colors render correctly (requires terminal with ANSI support)
- [ ] Summary flag works with all modes
- [ ] No git operations, only create files


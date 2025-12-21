---
name: test-diff-tree
description: Generate nested test files to QA hierarchical diff visualization
argument-hint: "[optional: file count, default 8]"
---

# Test git-diff-tree Visualization

Generate a nested directory structure with test files to validate the hierarchical diff tree output.

## Workflow

### Step 1: Create Test Directory Structure

Create nested directories that will produce an interesting tree:

```
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

### Step 2: Generate Files with Varying Sizes

Each file should have different line counts to test stat rendering:
- Small files: 5-10 lines
- Medium files: 20-40 lines
- Large files: 80-120 lines

Use realistic-looking placeholder content (TypeScript/markdown).

### Step 3: Run git-diff-tree Tests

After creating files, run both modes:

```bash
# Full tree view
$PROJECT_ROOT/bumper-lanes-plugin/bin/git-diff-tree

# Summary only
$PROJECT_ROOT/bumper-lanes-plugin/bin/git-diff-tree --summary
```

### Step 4: Report Results

Show the output from each mode and verify:

| Check | Expected |
|-------|----------|
| Tree structure | Directories shown with `/` suffix, proper `├──` and `└──` branches |
| Color output | Directories blue, additions green, deletions red |
| Stats alignment | `+N -M` shown for each file |
| Summary | Total lines and file count only |

### Step 5: Cleanup

Remove test files:

```bash
rm -rf tmp-diff-tree-test/
```

Or offer to delete them.

## Success Criteria

- [ ] Nested directories render with correct tree branches
- [ ] Files at different depths display properly
- [ ] No orphaned `│` pipes in tree output
- [ ] Root-level files and directories both render correctly
- [ ] Summary mode shows only totals

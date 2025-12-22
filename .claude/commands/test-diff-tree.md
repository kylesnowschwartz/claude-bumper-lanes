---
name: test-diff-tree
description: Generate nested test files to QA diff visualization modes (tree, sparkline, etc.)
argument-hint: "[optional: file count, default 8]"
---

# Test git-diff-tree Visualizations

Generate a nested directory structure with test files to validate all diff visualization modes (tree, collapsed, smart, topn).

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

### Step 2: Generate Files SEQUENTIALLY

**CRITICAL**: Create files ONE AT A TIME in sequence. After creating each file:
1. Run `$PROJECT_ROOT/bumper-lanes-plugin/bin/git-diff-tree` to show the updated tree
2. Pause briefly so the user can see the diff visualization change

Each file should have different line counts to test stat rendering:
- Small files: 5-10 lines
- Medium files: 20-40 lines
- Large files: 80-120 lines

Use realistic-looking placeholder content (TypeScript/markdown).

**Order of creation** (create one, show tree, repeat):
1. README.md (small)
2. src/index.ts (small)
3. src/utils/helpers.ts (medium)
4. src/components/Button.tsx (medium)
5. src/components/Modal.tsx (large)
6. tests/unit/button.test.ts (large)
7. Any additional files to reach target count

### Step 3: Ask User About Visualization Tests

After all files are created, **ask the user** which visualization modes they want to test.

Available modes (`--mode` flag):
| Mode | Description |
|------|-------------|
| `tree` | Full hierarchical tree with `├──` branches (default) |
| `collapsed` | Single-line grouped by top-level directory |
| `smart` | Depth-2 aggregated sparkline |
| `topn` | Top N files by change size (hotspots) |

Also available:
- `--summary` flag for totals only (works with any mode)

**Do NOT automatically run these** - wait for user to pick which modes to compare.

### Step 4: Report Results

For each mode tested, verify:

| Mode | Check |
|------|-------|
| `tree` | Proper `├──` and `└──` branches, directories blue with `/` suffix, stats aligned |
| `collapsed` | Single line, groups separated by `│`, file counts in parens |
| `smart` | Sparkline chars `▁▂▃▄▅▆▇█`, depth-2 aggregation labels |
| `topn` | Top N files with ratio bars (green/red add/del split) |
| `--summary` | Totals only, no file breakdown |

**Color conventions** (all modes):
- Green: additions
- Red: deletions
- Blue: directories

### Step 5: Cleanup

Remove test files:

```bash
rm -rf tmp-diff-tree-test/
```

Or offer to delete them.

## Success Criteria

- [ ] Sequential file creation shows diff growing incrementally
- [ ] Each visualization mode produces distinct, readable output
- [ ] Nested directories render correctly in tree mode (no orphaned `│` pipes)
- [ ] Sparkline modes scale appropriately to change magnitude
- [ ] Colors render correctly (requires terminal with ANSI support)
- [ ] Summary flag works with all modes

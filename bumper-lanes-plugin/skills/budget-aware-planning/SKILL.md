---
name: budget-aware-planning
description: >
  This skill should be used when planning implementation work in a repository
  where bumper-lanes enforces a review budget. Use before starting a multi-file
  change, a refactor, or any edit batch likely to exceed a few hundred changed
  lines, and whenever a bumper-lanes budget message appears in context.
  Keywords: review budget, diff threshold, bumper-lanes, increment size,
  scope planning.
---

# Budget-Aware Planning

This repository meters your working-tree diff. Every added line spends points
from a review budget (edits 1.3× weight, new files 1.0×, deletions free, plus
a penalty for touching many files). When the budget runs out, your turn is
blocked until a human reviews and resets. The budget is a scope contract:
plan increments that fit it, rather than working until the meter trips.

## Before large edits

Check the remaining budget before starting a multi-file change:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/bumper-lanes status --widget=indicator
```

Output like `active (450/600 - 75%)` means 150 points (~110 edited lines)
remain. Size the next increment to finish inside that remainder, including
tests.

## Planning rules

1. Decompose work so each increment lands within one budget: implement,
   test, and stop at a reviewable boundary before the meter trips.
2. When a plan will not fit the remaining budget, say so and ask the user
   whether to pause for review now or split the plan — do not spend the
   budget and let the trip decide.
3. Deletions are free. Prefer removing code over adding it.
4. Do not work around the meter: committing solely to reset the budget, or
   batching edits into fewer, larger writes, defeats the review contract.

## When the meter trips

Stop adding scope. Summarize what changed and why at the level a reviewer
needs (decisions and risky spots, not a line-by-line recap), then wait.
The user runs `/bumper-reset` after review to restore the budget.

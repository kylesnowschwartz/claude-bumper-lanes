---
description: Load the bumper-lanes development framework and skills into context for building
allowed-tools: Bash(git log:*), Bash(bd:*), Bash(eza:*), Bash(git branch:*), Skill
model: haiku
---

!`git branch --show-current`
!`git log -5 --oneline`
!`eza --tree --level 5`
!`bd ready`

Invoke these skills to load their knowledge into context:
- Skill(effective-go)
- Skill(hud-first)
- Skill(plugin-dev:plugin-dev-guide)

Then:
1. If there's an active epic in ready work, show its dependency tree with `bd dep tree <epic-id> --direction=up`
2. Acknowledge what we're working on based on the plan
3. Suggest claiming the next ready task or continuing in-progress work
4. Wait for user feedback before starting implementation


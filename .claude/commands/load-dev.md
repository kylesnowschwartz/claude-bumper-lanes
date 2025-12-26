---
description: Load the bumper-lanes development framework and skills into context for building
allowed-tools: Bash(git log:*), Bash(bd:*), Bash(eza:*), Bash(git branch:*), Skill
model: haiku
---

Invoke these skills to load their knowledge into context:
- Skill(effective-go)
- Skill(hud-first)
- Skill(plugin-dev:plugin-dev-guide)

A series of bash commands will automatically run to gather context about the current development environment. Loading...
!`git branch --show-current`
!`git log -5 --oneline`
!`eza --tree --level 5`
!`bd ready`

Then:
1. If there's an active epic in ready work, show its dependency tree with `bd dep tree <epic-id> --direction=up`
2. Acknowledge what we're working on based on the plan
3. Check if one of our several sequence diagrams is relevant to the current work and read it in context
4. Suggest claiming the next ready task or continuing in-progress work
5. Wait for user feedback before starting implementation


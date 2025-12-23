---
description: Load the bumper-lanes development framework and skills into context for building
allowed-tools: Bash(git log:*), Bash(bd:*), Bash(eza:*), Bash(git branch:*), Skill

---

Current branch:
!`git branch --show-current`

Check for ready work:
!`bd ready`

Review what was just accomplished:
!`git log -5 --oneline`

Review the code structure:
!`eza --tree --level 5 --git-ignore bumper-lanes-plugin/tools/diff-viz`

Then, invoke these skills to load their knowledge into context:
- Skill(effective-go)
- Skill(hud-first)
- Skill(plugin-dev)

Then:
1. If there's an active epic in ready work, show its dependency tree with `bd dep tree <epic-id>`
2. Acknowledge what we're working on based on the plan
3. Suggest claiming the next ready task or continuing in-progress work
4. Wait for user feedback before starting implementation


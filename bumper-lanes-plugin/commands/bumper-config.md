---
description: Show or set bumper lanes threshold configuration
argument-hint: "[show|set <threshold>|personal <threshold>]"
---

/claude-bumper-lanes:bumper-config

Configuration command handled by the UserPromptSubmit hook.

Usage:
- `/bumper-config` or `/bumper-config show` - Display current configuration
- `/bumper-config set 300` - Set repo threshold (creates .bumper-lanes.json)
- `/bumper-config personal 500` - Set personal threshold (in .git/bumper-config.json, untracked)

Additional user arguments: $ARGUMENTS

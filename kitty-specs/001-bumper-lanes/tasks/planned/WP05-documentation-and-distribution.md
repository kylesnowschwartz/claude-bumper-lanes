---
work_package_id: "WP05"
subtasks:
  - "T015"
  - "T016"
title: "Documentation and Distribution"
phase: "Phase 2 - Polish"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
history:
  - timestamp: "2025-11-02T20:56:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
  - timestamp: "2025-11-02T21:31:00Z"
    lane: "planned"
    agent: "claude"
    shell_pid: ""
    action: "Rewritten after user corrections (directory paths, removed plugin validation subtask for MVP)"
---

# Work Package Prompt: WP05 – Documentation and Distribution

## Objectives & Success Criteria

- Create comprehensive README with installation, usage, troubleshooting
- Document complete directory structure and plugin architecture
- Enable new developers to install and use plugin in under 10 minutes (per success criteria SC-007)

**Success Metric**: New developer can follow README to install plugin locally, trigger threshold block, run `/bumper-reset`, and understand how the plugin works without external help.

## Context & Constraints

**Prerequisites**:
- WP01-WP04 complete (all implementation finished)

**Supporting Documents**:
- `kitty-specs/001-bumper-lanes/quickstart.md` - Detailed installation and usage guide (reference source for README content)
- `kitty-specs/001-bumper-lanes/spec.md` - Success Criteria SC-007 (documentation goal)

**Architectural Decisions**:
- **README focus**: High-level overview with clear examples
- **Marketplace config**: Already created in WP01 T004
- **Validation**: Plugin structure validation done in WP01, no separate task needed

## Subtasks & Detailed Guidance

### Subtask T015 – Create `README.md`

**Purpose**: Provide project overview, installation instructions, quick start, and troubleshooting guide.

**Steps**:
1. Create `README.md` file in repository root
2. Structure sections:
   - **Title and Description** (what is Bumper Lanes, why use it)
   - **Features** (automatic enforcement, consent mechanism, configurable thresholds for v2)
   - **Installation** (local development method for MVP)
   - **Quick Start** (minimal example to trigger threshold)
   - **Configuration** (hardcoded 300 for MVP, configurable in v2)
   - **Commands** (`/bumper-reset` overview)
   - **Architecture** (directory structure, hook lifecycle)
   - **Troubleshooting** (common issues and fixes)
   - **Development** (how to contribute, prerequisites)
   - **License** (MIT)

3. **Title Section**:
   ```markdown
   # Claude Bumper Lanes

   A Claude Code plugin that enforces git diff thresholds to promote disciplined code review during AI agent sessions.

   **Key Features**:
   - 🚦 Automatic threshold enforcement (300 lines changed triggers block)
   - 📊 Real-time diff statistics tracking
   - 🔄 Explicit consent mechanism via `/bumper-reset` command
   - 🔒 Non-destructive (preserves git index state)
   - ⚡ Low overhead (<500ms per agent stop)

   ---
   ```

4. **Installation Section**:
   ```markdown
   ## Installation

   ### Local Development (MVP)

   1. Clone the repository:
      ```bash
      git clone https://github.com/kylesnowschwartz/claude-bumper-lanes.git
      cd claude-bumper-lanes
      ```

   2. Add local marketplace:
      ```bash
      # In Claude Code session, from repository root
      /plugin marketplace add .
      ```

   3. Install plugin:
      ```bash
      /plugin install bumper-lanes@local-dev
      ```

   4. Verify installation:
      ```bash
      /help  # Look for /bumper-reset command
      ```

   ### Prerequisites

   - Claude Code installed
   - Git 2.x+
   - Bash 4.0+
   - `jq` command-line JSON processor

   ---
   ```

5. **Quick Start Section**:
   ```markdown
   ## Quick Start

   1. **Start Claude Code session** in a git repository:
      ```bash
      cd /path/to/your/project
      claude-code
      ```

   2. **Make changes with Claude** (incrementally across multiple stops):
      ```
      You: Add a new user authentication feature
      Claude: [makes code changes...]
      ```

   3. **Hit threshold** (when cumulative changes exceed 300 lines):
      ```
      ⚠ Diff threshold exceeded: 430/300 lines changed (143%).

      Changes:
        8 files changed, 287 insertions(+), 143 deletions(-)

      Review your changes and run /bumper-reset to continue.
      ```

   4. **Review and reset** (after examining changes):
      ```bash
      git status
      git diff
      ```

      ```
      You: /bumper-reset
      ```

      ```
      ✓ Baseline reset complete.

      Previous baseline: 3a4b5c6 (captured 2025-11-02 20:15:00)
      New baseline: 1f2e3d4 (captured 2025-11-02 20:45:30)

      Changes accepted: 8 files, 287 insertions(+), 143 deletions(-) [430 lines total]

      You now have a fresh diff budget of 300 lines. Pick up where we left off?
      ```

   5. **Continue coding** with fresh threshold budget.

   ---
   ```

6. **Architecture Section**:
   ```markdown
   ## Architecture

   ### Directory Structure

   ```
   claude-bumper-lanes/              # Repository root
   ├── .claude-plugin/
   │   └── marketplace.json         # Local marketplace config
   ├── bumper-lanes-plugin/         # Plugin root (CLAUDE_PLUGIN_ROOT)
   │   ├── .claude-plugin/
   │   │   └── plugin.json          # Plugin manifest
   │   ├── hooks/
   │   │   ├── hooks.json           # Hook registration
   │   │   ├── entrypoints/         # Hook scripts
   │   │   │   ├── session-start.sh # Capture baseline
   │   │   │   ├── stop.sh          # Check threshold
   │   │   │   └── reset-baseline.sh # Reset baseline (called by /bumper-reset)
   │   │   └── lib/                 # Library functions
   │   │       ├── git-state.sh     # Git tree operations
   │   │       ├── state-manager.sh # Session state management
   │   │       └── threshold.sh     # Threshold calculation
   │   └── commands/
   │       └── bumper-reset.md      # /bumper-reset command spec
   └── README.md
   ```

   ### Hook Lifecycle

   1. **SessionStart**: Captures baseline git tree SHA on Claude Code session start
   2. **Stop**: Checks diff threshold when main agent stops, blocks if exceeded
   3. **User runs `/bumper-reset`**: Executes `reset-baseline.sh` to update baseline
   4. **Repeat**: Threshold tracking continues with new baseline

   ### Threshold Calculation

   - **Metric**: Simple line count (additions + deletions)
   - **Limit**: 300 lines (hardcoded for MVP)
   - **Non-destructive**: Uses temporary git index to avoid modifying staging area

   ---
   ```

7. **Troubleshooting Section**:
   ```markdown
   ## Troubleshooting

   ### Plugin Not Working

   **Check hooks are registered**:
   ```bash
   cat bumper-lanes-plugin/hooks/hooks.json
   ```

   **Check scripts are executable**:
   ```bash
   ls -l bumper-lanes-plugin/hooks/entrypoints/*.sh
   # Should show -rwxr-xr-x permissions
   ```

   **Check session state exists**:
   ```bash
   ls .git/bumper-checkpoints/session-*
   ```

   ### Baseline Not Captured

   **Symptom**: `/bumper-reset` says "No active session found"

   **Fix**: Exit and restart Claude Code session (triggers SessionStart hook).

   ### Threshold Not Blocking

   **Symptom**: Large changes made but no block message

   **Debug**:
   ```bash
   # Check session state exists
   ls .git/bumper-checkpoints/session-*

   # Check baseline value
   cat .git/bumper-checkpoints/session-* | jq .
   ```

   ### Git Errors

   **Common Causes**:
   - Not in a git repository (plugin auto-disables)
   - Git repo corrupted (run `git fsck`)
   - Permissions issue (check `.git/` is writable)

   ---
   ```

8. **Development Section**:
   ```markdown
   ## Development

   ### Contributing

   1. Fork the repository
   2. Create a feature branch
   3. Make changes
   4. Submit pull request

   ### Testing

   See `kitty-specs/001-bumper-lanes/quickstart.md` for detailed test scenarios.

   ---
   ```

9. **License Section**:
   ```markdown
   ## License

   MIT License - see LICENSE file for details.

   ---

   ## Links

   - **Quickstart Guide**: [kitty-specs/001-bumper-lanes/quickstart.md](kitty-specs/001-bumper-lanes/quickstart.md)
   - **Data Model**: [kitty-specs/001-bumper-lanes/data-model.md](kitty-specs/001-bumper-lanes/data-model.md)
   - **Repository**: https://github.com/kylesnowschwartz/claude-bumper-lanes
   - **Issues**: https://github.com/kylesnowschwartz/claude-bumper-lanes/issues
   ```

**Files**: `README.md` (repository root)

**Parallel?**: Can be written alongside T016.

**Notes**:
- README is high-level overview, quickstart.md is comprehensive guide
- Installation section documents local development method only (MVP)
- Quick start shows complete workflow from session start to reset
- Architecture section documents directory structure and hook lifecycle
- Troubleshooting section covers common issues from quickstart.md
- Development section points to quickstart.md for detailed testing
- Link to quickstart.md for developers wanting more detail

### Subtask T016 – Verify marketplace configuration

**Purpose**: Confirm marketplace configuration created in WP01 T004 is correct.

**Steps**:
1. Verify `.claude-plugin/marketplace.json` exists at repository root
2. Validate JSON syntax with `jq . .claude-plugin/marketplace.json`
3. Confirm structure matches WP01 T004 specification:
   ```json
   {
     "name": "local-dev",
     "owner": {
       "name": "Kyle Snow Schwartz",
       "email": "kyle@kylesnowschwartz.com"
     },
     "description": "Local marketplace for Bumper Lanes plugin development",
     "version": "1.0.0",
     "plugins": [
       {
         "name": "bumper-lanes",
         "version": "1.0.0",
         "source": "./bumper-lanes-plugin",
         "description": "Git diff threshold enforcement for disciplined vibe coding"
       }
     ]
   }
   ```
4. Test installation:
   ```bash
   # In Claude Code session, from repository root
   /plugin marketplace add .
   /plugin install bumper-lanes@local-dev
   /help  # Verify /bumper-reset appears
   ```

**Files**: `.claude-plugin/marketplace.json` (already created in WP01 T004)

**Parallel?**: Can be done alongside T015 (README writing).

**Notes**:
- This is a verification task, not a creation task (file already exists from WP01)
- Marketplace config lives at repository root (NOT inside bumper-lanes-plugin/)
- `source: "./bumper-lanes-plugin"` points to plugin subdirectory
- Developer uses `/plugin marketplace add .` from repo root to register marketplace
- Then installs with `/plugin install bumper-lanes@local-dev`

## Test Strategy

**No automated tests required** - manual validation sufficient.

**Manual Validation**:
1. New developer reads README (timed - should take <10 minutes to understand)
2. Developer follows installation steps
3. Developer triggers threshold and resets successfully
4. Developer understands plugin architecture from README

**Documentation Review**:
1. README is concise (under 300 lines, not exhaustive)
2. Installation instructions are copy-paste ready
3. Quick Start example shows actual command output
4. Architecture section clearly explains directory structure and hook lifecycle
5. Troubleshooting commands are tested and work
6. All referenced files exist (no broken links)

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| README too long (walls of text) | Keep concise, link to quickstart.md for details |
| Installation steps unclear | Test on clean environment before finalizing |
| Prerequisites missing | List all prerequisites clearly (Git, Bash, jq) |
| Links break after repository restructure | Use relative paths for in-repo links |
| Marketplace config wrong path | Already created in WP01, just verify in T016 |

## Definition of Done Checklist

- [ ] `README.md` created with all required sections (installation, quick start, architecture, troubleshooting)
- [ ] Marketplace config verified (already exists from WP01 T004)
- [ ] README links to quickstart.md and other documentation
- [ ] New developer can install and use plugin in <10 minutes (manual test)
- [ ] `/help` command shows `/bumper-reset` in Claude Code session after installation
- [ ] `tasks.md` WP05 checkbox marked complete

## Review Guidance

**Acceptance Checkpoints**:
1. README is comprehensive but concise (under 300 lines)
2. Installation section documents local development method
3. Quick Start section shows complete workflow (session → block → reset → continue)
4. Architecture section documents directory structure and hook lifecycle
5. Troubleshooting section covers common issues from quickstart.md
6. Marketplace config verified (correct format, exists at repo root)

**What to verify**:
- README is under 300 lines (concise, not exhaustive)
- Installation instructions are copy-paste ready
- Quick Start example shows actual command output
- Architecture section includes directory structure diagram
- Troubleshooting commands are tested and work
- Marketplace config matches WP01 T004 specification
- All referenced files exist (no broken links)

## Activity Log

- 2025-11-02T20:56:00Z – system – lane=planned – Prompt created via /spec-kitty.tasks.
- 2025-11-02T21:31:00Z – claude – lane=planned – Rewritten after user corrections (directory paths, removed plugin validation subtask for MVP).

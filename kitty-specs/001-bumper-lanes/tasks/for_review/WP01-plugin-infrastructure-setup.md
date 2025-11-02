---
work_package_id: "WP01"
subtasks:
  - "T001"
  - "T002"
  - "T003"
title: "Plugin Infrastructure Setup"
phase: "Phase 0 - Setup"
lane: "for_review"
assignee: ""
agent: "claude"
shell_pid: "21511"
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
    action: "Rewritten after user corrections (directory structure, no config, no SubagentStop)"
---

# Work Package Prompt: WP01 – Plugin Infrastructure Setup

## Objectives & Success Criteria

- Create complete plugin directory structure following Claude Code plugin standards
- Establish plugin manifest (`bumper-lanes-plugin/.claude-plugin/plugin.json`) with correct metadata
- Register hook events (SessionStart, Stop) via `bumper-lanes-plugin/hooks/hooks.json`
- Create marketplace configuration for local development testing
- No configuration system for MVP (threshold hardcoded to 300 lines)

**Success Metric**: Plugin directory structure is correct, manifest and hooks.json are valid, marketplace config enables local testing.

## Context & Constraints

**Prerequisites**: None - this is the starting work package.

**Supporting Documents**:
- `kitty-specs/001-bumper-lanes/plan.md` - Project Structure section
- `kitty-specs/001-bumper-lanes/contracts/plugin-manifest.json` - Plugin manifest schema
- `kitty-specs/001-bumper-lanes/contracts/hooks-registration.json` - Hook registration schema
- Reference implementation: `/Users/kyle/Code/meta-claude/SimpleClaude/plugins/sc-hooks/`

**Architectural Constraints**:
- Repository root: `claude-bumper-lanes/`
- Plugin root: `claude-bumper-lanes/bumper-lanes-plugin/` (CLAUDE_PLUGIN_ROOT)
- Hook scripts in `bumper-lanes-plugin/hooks/entrypoints/`
- Library functions in `bumper-lanes-plugin/hooks/lib/`
- Commands in `bumper-lanes-plugin/commands/`
- Marketplace config at repo root: `claude-bumper-lanes/.claude-plugin/marketplace.json`
- All hook scripts require executable permissions (`chmod +x`)
- Plugin name: `bumper-lanes` (NOT `claude-bumper-lanes` - that's the repo name)
- Repository: `https://github.com/kylesnowschwartz/claude-bumper-lanes`
- Author: Kyle Snow Schwartz (kyle@kylesnowschwartz.com)

## Subtasks & Detailed Guidance

### Subtask T001 – Create `bumper-lanes-plugin/.claude-plugin/plugin.json` manifest

**Purpose**: Define plugin metadata per Claude Code plugin specification.

**Steps**:
1. Create `bumper-lanes-plugin/.claude-plugin/` directory
2. Create `plugin.json` file inside `.claude-plugin/` directory
3. Populate with minimal metadata (Claude Code discovers hooks/commands by convention):
   ```json
   {
     "name": "bumper-lanes",
     "version": "1.0.0",
     "description": "Enforces git diff thresholds to promote disciplined code review during AI agent sessions",
     "author": {
       "name": "Kyle Snow Schwartz",
       "email": "kyle@kylesnowschwartz.com"
     },
     "homepage": "https://github.com/kylesnowschwartz/claude-bumper-lanes",
     "repository": "https://github.com/kylesnowschwartz/claude-bumper-lanes",
     "license": "MIT",
     "keywords": ["git", "diff", "threshold", "code-review", "safety"]
   }
   ```
4. Validate JSON syntax with `jq . bumper-lanes-plugin/.claude-plugin/plugin.json`

**Files**: `bumper-lanes-plugin/.claude-plugin/plugin.json`

**Parallel?**: Can proceed alongside T002-T003.

**Notes**:
- Plugin name is `bumper-lanes` (repo name is `claude-bumper-lanes`)
- No `hooks` or `commands` fields needed - Claude Code discovers by directory convention
- Version 1.0.0 for MVP release

### Subtask T002 – Create `bumper-lanes-plugin/hooks/hooks.json` registration

**Purpose**: Map Claude Code hook events (SessionStart, Stop) to executable bash scripts.

**Steps**:
1. Create `bumper-lanes-plugin/hooks/` directory
2. Create `hooks.json` file inside `hooks/` directory
3. Populate with hook registrations (SessionStart and Stop only, no SubagentStop):
   ```json
   {
     "description": "Bumper Lanes hook configuration for threshold enforcement",
     "hooks": {
       "SessionStart": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "${CLAUDE_PLUGIN_ROOT}/hooks/entrypoints/session-start.sh"
             }
           ]
         }
       ],
       "Stop": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "${CLAUDE_PLUGIN_ROOT}/hooks/entrypoints/stop.sh"
             }
           ]
         }
       ]
     }
   }
   ```
4. Validate JSON syntax with `jq . bumper-lanes-plugin/hooks/hooks.json`

**Files**: `bumper-lanes-plugin/hooks/hooks.json`

**Parallel?**: Can proceed alongside T001, T003.

**Notes**:
- `${CLAUDE_PLUGIN_ROOT}` resolves to `bumper-lanes-plugin/` at runtime
- SessionStart: Capture baseline tree on session initialization
- Stop: Check threshold on main agent stop, block if exceeded
- **No SubagentStop** - subagents are unpredictable, excluded from MVP
- Script paths will be created in WP03

### Subtask T003 – Create directory structure

**Purpose**: Establish directories for hook scripts, library functions, and slash commands.

**Steps**:
1. Create directory structure:
   ```
   bumper-lanes-plugin/
   ├── .claude-plugin/          # (Created in T001)
   ├── hooks/
   │   ├── hooks.json           # (Created in T002)
   │   ├── entrypoints/         # Hook entry scripts
   │   └── lib/                 # Reusable bash libraries
   └── commands/                # Slash command markdown files
   ```
2. Execute mkdir commands:
   ```bash
   cd bumper-lanes-plugin
   mkdir -p hooks/entrypoints
   mkdir -p hooks/lib
   mkdir -p commands
   ```
3. Verify structure:
   ```bash
   eza --tree --level 3 bumper-lanes-plugin
   ```

**Files**: Directories only (no files created in this subtask).

**Parallel?**: Can proceed alongside T001-T002, but those tasks depend on directories existing.

**Notes**:
- `hooks/entrypoints/` contains main hook scripts (session-start.sh, stop.sh, reset-baseline.sh)
- `hooks/lib/` contains reusable bash library functions (git-state.sh, state-manager.sh, threshold.sh)
- `commands/` contains markdown files for slash commands (bumper-reset.md)
- No `config/` directory - threshold is hardcoded for MVP

### Subtask T004 – Create `.claude-plugin/marketplace.json` for local development

**Purpose**: Enable local marketplace testing without publishing to GitHub.

**Steps**:
1. Create `.claude-plugin/` directory at repository root (NOT inside bumper-lanes-plugin)
2. Create `marketplace.json` file:
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
3. Validate JSON syntax with `jq . .claude-plugin/marketplace.json`

**Files**: `.claude-plugin/marketplace.json` (at repo root)

**Parallel?**: Can proceed alongside T001-T003.

**Notes**:
- Marketplace config lives at repo root, NOT inside plugin directory
- `source: "./bumper-lanes-plugin"` points to plugin subdirectory
- Developer uses `/plugin marketplace add .` from repo root to register marketplace
- Then installs with `/plugin install bumper-lanes@local-dev`
- Marketplace config is separate from plugin manifest

## Test Strategy

**No explicit tests required** - plugin structure validation serves as functional test.

**Manual Validation**:
1. Run `claude plugin validate bumper-lanes-plugin` from repository root
2. Verify output shows no errors
3. Check that all required files exist and are valid JSON
4. Verify directory structure matches specification

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Invalid JSON syntax breaks plugin loading | Validate all JSON files with `jq` before committing |
| Plugin manifest path wrong | Use `bumper-lanes-plugin/.claude-plugin/plugin.json`, NOT root |
| Marketplace source path wrong | Use `./bumper-lanes-plugin` relative to repo root |
| Hook script paths don't resolve | Use `${CLAUDE_PLUGIN_ROOT}` consistently |
| Directory structure doesn't match reference | Compare with `/Users/kyle/Code/meta-claude/SimpleClaude/plugins/sc-hooks/` |

## Definition of Done Checklist

- [ ] `bumper-lanes-plugin/.claude-plugin/plugin.json` exists with correct metadata
- [ ] `bumper-lanes-plugin/hooks/hooks.json` exists with SessionStart and Stop registered
- [ ] `.claude-plugin/marketplace.json` exists at repo root with correct source path
- [ ] Directory structure complete: `hooks/entrypoints/`, `hooks/lib/`, `commands/`
- [ ] All JSON files validated with `jq`
- [ ] `claude plugin validate bumper-lanes-plugin` passes
- [ ] `tasks.md` WP01 checkbox marked complete

## Review Guidance

**Acceptance Checkpoints**:
1. Plugin manifest at `bumper-lanes-plugin/.claude-plugin/plugin.json` (NOT repo root)
2. Hooks registration at `bumper-lanes-plugin/hooks/hooks.json`
3. Marketplace config at `.claude-plugin/marketplace.json` (repo root)
4. Only SessionStart and Stop hooks registered (no SubagentStop)
5. Marketplace source points to `./bumper-lanes-plugin`
6. Directory structure matches reference implementation

**What to verify**:
- JSON files parse successfully (no syntax errors)
- Plugin name is `bumper-lanes` (not `claude-bumper-lanes`)
- Hook script paths use `${CLAUDE_PLUGIN_ROOT}/hooks/entrypoints/`
- Marketplace source path is relative: `./bumper-lanes-plugin`
- No config directory (threshold hardcoded in WP03)

## Activity Log

- 2025-11-02T20:56:00Z – system – lane=planned – Prompt created via /spec-kitty.tasks.
- 2025-11-02T21:31:00Z – claude – lane=planned – Rewritten after user corrections (directory structure, no config, no SubagentStop).
- 2025-11-02T09:05:55Z – claude – shell_pid=21511 – lane=doing – Started WP01: Plugin Infrastructure Setup
- 2025-11-02T09:07:22Z – claude – shell_pid=21511 – lane=for_review – Completed WP01: All 4 subtasks implemented and validated

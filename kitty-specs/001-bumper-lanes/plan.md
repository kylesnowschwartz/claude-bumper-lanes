# Implementation Plan: Bumper Lanes
*Path: [kitty-specs/001-bumper-lanes/plan.md](kitty-specs/001-bumper-lanes/plan.md)*


**Branch**: `001-bumper-lanes` | **Date**: 2025-11-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/kitty-specs/001-bumper-lanes/spec.md`

**Note**: This template is filled in by the `/spec-kitty.plan` command. See `.kittify/templates/commands/plan.md` for the execution workflow.

The planner will not begin until all planning questions have been answered—capture those answers in this document before progressing to later phases.

## Summary

Bumper Lanes is a Claude Code plugin that prevents uncontrolled code changes by tracking git diff statistics during agent sessions and blocking execution when cumulative changes exceed a configurable threshold. The plugin enforces disciplined "vibe coding" by requiring explicit developer review (`/bumper-reset` command) before allowing continued code generation.

**Technical approach**: Hook-based enforcement using SessionStart (baseline capture), Stop/SubagentStop (threshold checking), and UserPromptSubmit (consent detection). State tracking uses git's tree object model with temporary index files to capture working tree snapshots without creating commits. Diff statistics computed via `git diff-tree` between baseline and current tree objects.

## Technical Context

**Language/Version**: Bash 4.0+ (Claude Code hook scripts are bash-based)
**Primary Dependencies**: Git 2.x+, jq (JSON parsing), Claude Code hooks system
**Storage**: Filesystem (`.git/bumper-checkpoints/` directory for session state)
**Testing**: Bats (Bash Automated Testing System) for hook script testing
**Target Platform**: macOS/Linux (Claude Code supported platforms)
**Project Type**: Single project (Claude Code plugin with bash hook scripts)
**Performance Goals**: <500ms overhead per Stop hook invocation, <2s for tree capture in repos with <100k files
**Constraints**: Non-destructive (must preserve git index state), PID-isolated (concurrent sessions in same repo), no commits created
**Scale/Scope**: Support repos up to 1M LOC, handle up to 10k files changed per session

**Architecture Decisions** (from planning interrogation):
- **Baseline persistence**: Git tree objects via temporary index file (`GIT_INDEX_FILE`) approach
- **Threshold metric**: Simple line count (default: 300 total lines changed), config structure ready for future weighting
- **Consent mechanism**: `/bumper-reset` slash command (explicit, idempotent)
- **Multi-session isolation**: PID-based state files (`.git/bumper-checkpoints/baseline-$$`)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Status**: No project constitution defined yet (`.kittify/memory/constitution.md` is template).

**Default principles applied**:
- ✅ **Simplicity**: Plugin uses bash scripts (minimal dependencies), simple line-count threshold for MVP
- ✅ **Non-destructive**: Git operations preserve index state via temporary index approach
- ✅ **Testability**: Bash scripts are unit-testable with Bats framework
- ✅ **Reliability**: PID-based isolation prevents session interference

**No violations** - proceeding to Phase 0.

## Project Structure

### Documentation (this feature)

```
kitty-specs/[###-feature]/
├── plan.md              # This file (/spec-kitty.plan command output)
├── research.md          # Phase 0 output (/spec-kitty.plan command)
├── data-model.md        # Phase 1 output (/spec-kitty.plan command)
├── quickstart.md        # Phase 1 output (/spec-kitty.plan command)
├── contracts/           # Phase 1 output (/spec-kitty.plan command)
└── tasks.md             # Phase 2 output (/spec-kitty.tasks command - NOT created by /spec-kitty.plan)
```

### Source Code (repository root)

```
claude-bumper-lanes/                # Plugin root
├── .claude-plugin/                 # Plugin configuration (REQUIRED)
│   └── plugin.json                 # Plugin manifest
├── README.md                       # Installation and usage docs
├── hooks/                          # Hook configuration and scripts
│   ├── hooks.json                  # Hook registration (maps events to scripts)
│   └── scripts/                    # Hook script implementations
│       ├── session-start.sh        # Captures baseline tree on session start
│       ├── stop.sh                 # Checks threshold on agent stop
│       ├── user-prompt-submit.sh   # Detects /bumper-reset consent command
│       └── lib/                    # Shared hook utilities
│           ├── git-state.sh        # Tree capture and diff functions
│           ├── threshold.sh        # Threshold computation logic
│           └── state-manager.sh    # PID-based state file management
├── commands/                       # Slash command definitions (markdown with frontmatter)
│   ├── bumper-reset.md             # /bumper-reset command spec
│   └── bumper-status.md            # /bumper-status command spec (optional)
├── config/                         # Configuration templates
│   └── bumper-lanes.json           # Default threshold and settings
└── tests/                          # Test suite
    ├── unit/                       # Unit tests for bash functions
    │   ├── git-state.bats
    │   ├── threshold.bats
    │   └── state-manager.bats
    ├── integration/                # End-to-end hook tests
    │   ├── session-flow.bats
    │   └── consent-flow.bats
    └── fixtures/                   # Test repositories and data
        └── sample-repos/
```

**Structure Decision**: Standard Claude Code plugin structure. Plugin manifest in `.claude-plugin/plugin.json` (required directory name). Hook configuration in `hooks/hooks.json` maps events (SessionStart, Stop, etc.) to executable scripts. Hook scripts use `${CLAUDE_PLUGIN_ROOT}` environment variable for path resolution. Slash commands are markdown files with frontmatter in `commands/` directory.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

**N/A** - No constitutional violations. Design follows simplicity principles with bash scripts, minimal dependencies, and straightforward architecture.

---

## Phase 0: Research - COMPLETE ✓

**Completed**: 2025-11-02

**Artifacts Generated**:
- `research.md`: Complete research findings on git state tracking methods, hook system architecture, and threshold design decisions
- `data-model.md`: Entity definitions for Session State, Diff Statistics, Threshold Configuration, Block Events, and Tree Snapshots
- `research/source-register.csv`: 9 sources documented (Stack Overflow, Git docs, Claude Code docs)
- `research/evidence-log.csv`: 10 evidence entries linking findings to decisions

**Key Decisions Documented**:
1. Baseline persistence via git tree objects (temporary index approach)
2. Diff computation via `git diff-tree` with --numstat/--shortstat flags
3. Hook lifecycle: SessionStart → Stop/SubagentStop → UserPromptSubmit
4. PID-based state isolation for concurrent sessions
5. Simple line-count threshold metric for MVP

**Outstanding Questions**: None - all planning questions resolved

---

## Phase 1: Design & Contracts - COMPLETE ✓

**Completed**: 2025-11-02

**Artifacts Generated**:
- `data-model.md`: Complete entity model with validation rules, state transitions, relationships
- `contracts/session-start-hook.json`: SessionStart hook stdin/stdout contract
- `contracts/stop-hook.json`: Stop/SubagentStop hook contract with block response format
- `contracts/user-prompt-submit-hook.json`: UserPromptSubmit hook contract for /bumper-reset
- `contracts/plugin-manifest.json`: `.claude-plugin/plugin.json` manifest schema
- `contracts/hooks-registration.json`: `hooks/hooks.json` registration schema
- `quickstart.md`: Complete developer onboarding guide with installation, usage, troubleshooting

**Agent Context Updated**:
- `CLAUDE.md` updated with Bash 4.0+, Git 2.x+, jq, Claude Code hooks system
- Technology stack registered for downstream task generation

**Architecture Validated**:
- Corrected plugin structure to use `.claude-plugin/` directory (mandatory)
- Hook scripts registered via `hooks/hooks.json` with `${CLAUDE_PLUGIN_ROOT}` paths
- Slash commands defined as markdown files with frontmatter

---

## Next Steps

**Phase 2**: Run `/spec-kitty.tasks` to generate implementation tasks from this plan.

**Ready for Implementation**: All planning and design complete. Task generation can proceed.

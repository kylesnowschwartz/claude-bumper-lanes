# Plugin Specification: Bumper Lanes
<!-- Replace [FEATURE NAME] with the confirmed friendly title generated during /spec-kitty.specify. -->

**Feature Branch**: `001-bumper-lanes`
**Created**: 2025-11-01
**Status**: Draft
**Input**: User description: "claude-bumper-lanes is a claude-code plugin. It distributes a series of hooks that, on agent stop, checks the git diff stats. If it exceeds a certain threshold, it stops the user from proceeding to the next step, until they've indicated they've removed the git changes with explicit consent. This will 'reset' the git diff stat checker. (tracking diff stats between points in time without making commits will be a technical challenge)"

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.

  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Automatic Diff Threshold Enforcement (Priority: P1) <¯ MVP

As a developer using Claude Code, when Claude/subagents stop after making code changes, the plugin automatically checks the cumulative git diff statistics. If the changes exceed a configured threshold, Claude is blocked from continuing and I receive a clear explanation of what changed and why progression was blocked.

**Why this priority**: This is the core safety railwithout automatic enforcement, the plugin provides no value. This must work before any other features.

**Independent Test**: Install plugin in a test project, have Claude make incremental changes across multiple files until threshold is exceeded. Plugin should block with clear message showing diff stats and threshold limit.

**Acceptance Scenarios**:

1. **Given** plugin is installed and baseline is set, **When** Claude makes changes under threshold, **Then** Claude stops normally without interruption
2. **Given** plugin is installed and baseline is set, **When** Claude makes changes exceeding threshold (cumulative across multiple stops), **Then** Claude is blocked with message showing current diff stats, threshold limit, and instructions
3. **Given** plugin is installed but no baseline exists, **When** Claude attempts to stop, **Then** plugin establishes current HEAD as baseline and allows stop
4. **Given** plugin is active and threshold exceeded, **When** Claude tries to continue, **Then** block persists until user provides consent

---

### User Story 2 - Consent and Baseline Reset (Priority: P1) <¯ MVP

As a developer who has been blocked by the plugin, I can explicitly provide consent to accept the current changes. This consent action resets the baseline to the current state and allows Claude to continue working from this new checkpoint.

**Why this priority**: Blocking without an escape hatch makes the plugin unusable. Developers need a clear, quick way to acknowledge changes and continue.

**Independent Test**: Trigger a threshold block, execute the consent mechanism (command, flag, or prompt), verify baseline resets and Claude can proceed with fresh diff budget.

**Acceptance Scenarios**:

1. **Given** Claude is blocked due to threshold, **When** I provide explicit consent, **Then** baseline resets to current git state and block is lifted
2. **Given** baseline has been reset via consent, **When** Claude makes new changes, **Then** diff stats are measured from the new baseline
3. **Given** Claude is blocked, **When** I provide consent, **Then** I receive confirmation showing old baseline, new baseline, and total changes accepted
4. **Given** Claude is not blocked, **When** I attempt consent command, **Then** plugin reports no active block and shows current diff stats

---

### User Story 3 - Revert to Baseline (Priority: P2)

As a developer who has been blocked, I can choose to revert uncommitted changes back to the baseline instead of providing consent. This discards Claude's changes and resets the diff counter to zero.

**Why this priority**: Sometimes the agent drifts too far and the changes are wrong. Developers need a "reset" option that undoes work back to the last safe checkpoint.

**Independent Test**: Trigger a threshold block, execute revert command, verify working directory matches baseline and block is lifted.

**Acceptance Scenarios**:

1. **Given** Claude is blocked due to threshold, **When** I execute revert command, **Then** all changes since baseline are discarded and diff counter resets
2. **Given** Claude is blocked, **When** I confirm revert, **Then** plugin shows summary of reverted files before executing
3. **Given** revert operation fails (conflicts, staged changes), **When** revert is attempted, **Then** clear error message explains why and suggests next steps

---

### User Story 4 - Configurable Thresholds (Priority: P3)

As a developer, I can configure the diff threshold (lines added/changed, files touched, or weighted complexity) in project or user settings. Different projects may have different tolerance for change velocity.

**Why this priority**: Hardcoded thresholds won't fit all workflows. Large refactors need higher limits; sensitive projects need lower limits.

**Independent Test**: Configure custom threshold in settings, trigger changes that exceed old threshold but not new threshold, verify block/allow behavior matches new setting.

**Acceptance Scenarios**:

1. **Given** no custom threshold configured, **When** plugin initializes, **Then** reasonable default threshold is used
2. **Given** project-specific threshold configured, **When** plugin runs in that project, **Then** project threshold overrides user-level default
3. **Given** invalid threshold value, **When** plugin loads configuration, **Then** clear error message explains valid threshold formats and fallback behavior
4. **Given** threshold configuration changes, **When** Claude next stops, **Then** new threshold takes effect immediately

---

### User Story 5 - Status Visibility (Priority: P3)

As a developer, I can query the current diff stats, baseline commit/state, remaining threshold budget, and plugin status at any time without waiting for Claude to stop.

**Why this priority**: Developers need visibility into how close they are to threshold limits so they can decide when to checkpoint manually.

**Independent Test**: Run status command at various points during Claude session, verify accurate reporting of baseline, current diff, threshold, and percentage used.

**Acceptance Scenarios**:

1. **Given** plugin is active, **When** I run status command, **Then** output shows baseline reference, current diff stats, threshold limit, and percentage used
2. **Given** no baseline exists, **When** I run status command, **Then** output clearly states no baseline established yet
3. **Given** multiple git worktrees or repos, **When** status command runs, **Then** plugin correctly identifies and reports status for current working directory
4. **Given** plugin is disabled, **When** I run status command, **Then** clear message states plugin inactive

---

### Edge Cases

- What happens when plugin is installed in a non-git repository?
- How does plugin handle git worktrees, submodules, or monorepos?
- What if baseline commit/state is no longer accessible (force push, branch deleted)?
- How does plugin distinguish between Claude's changes vs manual user edits?
- What if user has staged changes or active merge/rebase when hook fires?
- How does plugin handle nested Claude sessions (subagents calling subagents)?
- What if consent is provided but network/disk operations fail during baseline update?
- How does plugin behave when git diff command fails or times out?
- What happens if multiple concurrent Claude sessions operate in same repository?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Plugin MUST register Stop and SubagentStop hooks in Claude Code hook system
- **FR-002**: Plugin MUST detect when operating in a git repository vs non-git directory
- **FR-003**: Plugin MUST track git diff statistics (lines added/removed/changed, files modified) against a baseline reference
- **FR-004**: Plugin MUST persist baseline reference state between hook invocations [NEEDS CLARIFICATION: mechanism TBD during planning - commit SHA, state file, stash, or hidden branch]
- **FR-005**: Plugin MUST compare cumulative diff stats against configured threshold when Stop/SubagentStop hooks fire
- **FR-006**: Plugin MUST block Claude from stopping (return `{"decision": "block"}`) when threshold is exceeded
- **FR-007**: Plugin MUST provide clear block message explaining current diff stats, threshold limit, and next steps (consent or revert)
- **FR-008**: Plugin MUST provide consent mechanism allowing developers to accept changes and reset baseline
- **FR-009**: Plugin MUST update baseline to current git state when consent is provided
- **FR-010**: Plugin MUST provide revert mechanism allowing developers to discard changes back to baseline
- **FR-011**: Plugin MUST respect `stop_hook_active` flag to prevent infinite loop when hook blocks and continues
- **FR-012**: Plugin MUST handle first-run scenario by establishing current HEAD as initial baseline
- **FR-013**: Plugin MUST allow threshold configuration via project-level or user-level settings
- **FR-014**: Plugin MUST provide status command showing current diff stats, baseline, threshold, and budget remaining
- **FR-015**: Plugin MUST fail gracefully when git commands error, timeout, or repository state is invalid
- **FR-016**: Plugin MUST distinguish between changes made during current Claude session vs pre-existing uncommitted changes
- **FR-017**: Plugin MUST provide clear error messages for edge cases (non-git repo, missing baseline, invalid config)
- **FR-018**: Plugin MUST document installation, configuration, and usage in plugin manifest and README

### Key Entities *(include if feature involves data)*

- **Baseline Reference**: Snapshot of repository state used as starting point for diff calculations. Represents the last "safe" checkpoint approved by developer. Must be retrievable even when no commits have been made.

- **Diff Stats**: Quantitative measure of changes between baseline and current state. Includes lines added, lines removed, lines changed, files modified. Used for threshold comparison.

- **Threshold Configuration**: User-configurable limits defining acceptable change magnitude before blocking. [NEEDS CLARIFICATION: metric definition TBD during planning - total lines, weighted complexity, file count, or combination]

- **Consent Record**: Event indicating developer explicitly accepted current changes. Triggers baseline update and diff counter reset.

- **Block State**: Active state when threshold exceeded. Persists across hook invocations until consent or revert action taken.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers installing the plugin experience automatic blocking when cumulative changes exceed threshold, with 100% reliability
- **SC-002**: Consent mechanism allows developers to acknowledge changes and resume work in under 10 seconds
- **SC-003**: Plugin introduces zero overhead to normal Claude workflow when changes are under threshold (no perceptible latency)
- **SC-004**: Status command provides accurate diff statistics with under 2-second response time for repositories with <100k files
- **SC-005**: Plugin correctly handles 95% of edge cases (non-git repos, missing baselines, git errors) with clear error messages rather than crashes
- **SC-006**: False positive rate (blocking when changes are reasonable) is under 5% with default threshold settings
- **SC-007**: Plugin documentation allows new users to install, configure, and understand core concepts in under 10 minutes

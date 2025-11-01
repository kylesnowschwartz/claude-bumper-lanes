<!--
Sync Impact Report
Version: 0.0.0 → 1.0.0 (Initial ratification - MAJOR bump for new governance)

Modified Principles: N/A (new constitution)

Added Sections:
  - Core Principles (5 principles: JSON-First, Template-Driven, Slash Command Orchestration, Test-After, Vertical Slice)
  - Workflow Enforcement (Phase Gates + Artifact Dependencies)
  - Governance (Amendment Procedure, Compliance Verification, Version History)

Removed Sections: N/A

Templates Updated:
  ✅ plan-template.md - Constitution Check section populated with concrete validation checklist
  ✅ spec-template.md - Already aligned (independently testable user stories match Principle V)
  ✅ tasks-template.md - Testing guidance updated to reflect Principle IV (Test-After Validation)
  ✅ commands/*.md - Validated: no agent-specific references requiring updates

Follow-up TODOs:
  - TODO(RATIFICATION_DATE): Set actual date when project officially adopts constitution (line 146)
  - Consider creating README.md with quickstart guide referencing constitution principles
-->

# Claude Bumper Lanes Constitution

## Core Principles

### I. JSON-First Configuration

All feature specifications, task definitions, and workflow orchestration MUST be expressed
through structured JSON or Markdown templates with YAML frontmatter. This ensures:
- Machine-readable specs that Claude Code agents can parse and validate
- Version-controlled documentation that serves as source of truth
- Automated workflow orchestration via slash commands
- Clear separation between structure (templates) and content (feature specs)

**Rationale**: JSON-based architecture enables programmatic validation, automated task
generation, and seamless integration with Claude Code's plugin marketplace while
maintaining human readability through Markdown.

### II. Template-Driven Development

Every workflow (specify, plan, research, tasks, review, accept) MUST follow a predefined
template that enforces consistent structure across features. Templates define:
- Required sections and optional sections (marked with NEEDS CLARIFICATION)
- Placeholder tokens (e.g., [FEATURE_NAME], [PROJECT_TYPE]) for systematic replacement
- Workflow gates that must pass before proceeding to next phase
- Cross-artifact dependencies and validation rules

**Rationale**: Templates eliminate ambiguity, reduce cognitive load on agents, and ensure
reproducible quality across all feature development cycles.

### III. Slash Command Orchestration

All complex workflows MUST be encapsulated as slash commands (e.g., `/spec-kitty.plan`,
`/spec-kitty.tasks`) that:
- Execute multi-step processes autonomously
- Validate prerequisites before beginning work
- Generate or update multiple artifacts atomically
- Report outcomes and next steps clearly

Simple operations (read file, run test) use direct tool calls; multi-phase workflows
require slash commands. No manual step-by-step execution for standardized processes.

**Rationale**: Slash commands provide ergonomic, repeatable workflows that hide complexity
while maintaining transparency through generated artifacts.

### IV. Test-After Validation

Testing is required but follows implementation. Each work package MUST include validation
criteria and test scenarios, executed after core functionality is complete:
- Contract tests verify API boundaries match specifications
- Integration tests validate cross-component interactions
- User scenarios from spec.md drive acceptance testing
- Tests added after feature completion; simple changes may use manual validation

**Rationale**: For AI-assisted development, iterative refinement with post-implementation
validation provides faster feedback loops than strict TDD while maintaining quality gates.

### V. Vertical Slice Isolation

Features MUST be organized as independently testable vertical slices:
- Each work package delivers end-to-end value (UI → logic → data if applicable)
- User stories prioritized (P1, P2, P3) and implemented as standalone slices
- Dependencies explicit; parallel work opportunities marked with [P] flag
- Changes localized within slice boundaries (high cohesion)

Apply Stable Dependencies Principle: slices point only at equal-or-more-stable neighbors.
Apply Information Hiding: encapsulate volatile design choices behind slice APIs.

**Rationale**: Vertical slicing enables incremental delivery, parallel development, and
localized changes while respecting domain boundaries from DDD principles.

## Workflow Enforcement

### Phase Gates

1. **Specification Gate** (`/spec-kitty.specify`): User scenarios, requirements, and
   success criteria MUST be complete before planning.

2. **Planning Gate** (`/spec-kitty.plan`): Constitution Check MUST pass (or violations
   justified) before research phase. Technical context MUST be clarified or marked
   [NEEDS CLARIFICATION].

3. **Task Gate** (`/spec-kitty.tasks`): Implementation plan with design artifacts
   (data-model.md, contracts/, quickstart.md) MUST exist before generating work packages.

4. **Review Gate** (`/spec-kitty.review`): Completed task prompt files MUST pass code
   review before transitioning to "done" kanban column.

5. **Acceptance Gate** (`/spec-kitty.accept`): All P1 user scenarios MUST pass validation
   before feature is merged to main.

### Artifact Dependencies

```
spec.md (user scenarios)
  ↓
plan.md (technical design)
  ↓
research.md + data-model.md + contracts/ + quickstart.md
  ↓
tasks.md (work packages)
  ↓
/tasks/planned/WPxx-*.md (executable prompts)
  ↓
/tasks/done/WPxx-*.md (reviewed implementations)
```

Each artifact MUST reference its inputs and validate against constitution principles
before creation.

## Governance

### Amendment Procedure

1. Proposed changes documented with rationale and version bump justification
2. Constitution version incremented per semantic versioning:
   - MAJOR: Principle removed/redefined (backward incompatible)
   - MINOR: Principle added or section materially expanded
   - PATCH: Clarifications, wording, non-semantic refinements
3. All dependent templates updated in same commit
4. Sync Impact Report prepended to constitution file

### Compliance Verification

- All `/spec-kitty.*` commands MUST validate against current constitution
- Plan template "Constitution Check" section enforces principle adherence
- Complexity violations MUST be justified in "Complexity Tracking" table
- Review command checks for principle violations before marking work complete

### Version History

**Version**: 1.0.0 | **Ratified**: TODO(RATIFICATION_DATE): Set when project officially adopts constitution | **Last Amended**: 2025-11-01

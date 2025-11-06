# Changelog

All notable changes to claude-bumper-lanes will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-11-06

### Added
- **Core threshold enforcement system**: Proactive blocking via PreToolUse hook for Write/Edit tools when cumulative diff exceeds 200-line threshold
- **Reactive stop enforcement**: Stop hook blocks Claude from finishing turn when threshold exceeded, forcing user review
- **Manual reset workflow**: `/claude-bumper-lanes:bumper-reset` command for explicit user approval after review
- **Weighted scoring system**: Delta tracking that correctly handles file deletions, additions, and modifications to prevent bypass scenarios
- **Session state management**: Per-session diff tracking with git tree snapshots for accurate cumulative measurement
- **Status line integration**: Real-time threshold status display in Claude Code status bar
- **Defense-in-depth architecture**: Multiple enforcement layers (PreToolUse, Stop, UserPromptSubmit) ensure changes cannot slip through
- **Comprehensive test suite**: BATS-based integration and unit tests covering all threshold scenarios
- **CI/CD pipeline**: GitHub Actions workflow for automated testing
- **Justfile test runner**: Convenient `just test` commands for local development

### Technical Details
- Bash 4.0+ implementation for maximum portability
- Git 2.x+ integration using `git write-tree` for baseline snapshots and `git diff-tree` for accurate diff calculation
- jq-based JSON state management for Claude Code hook I/O
- Fail-open error handling (availability over strictness)

### Documentation
- Architecture flow diagrams in Mermaid format
- Hook exit code reference documentation
- Comprehensive README with installation and usage instructions
- Inline code documentation explaining design decisions

[1.0.0]: https://github.com/kylesnowschwartz/claude-bumper-lanes/releases/tag/v1.0.0

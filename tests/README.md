# Claude Bumper Lanes Test Suite

Comprehensive test suite for the Claude Bumper Lanes plugin using the **Bats (Bash Automated Testing System)** framework.

## Quick Start

```bash
# Run all tests
just test

# Run only unit tests
just test-unit

# Run only integration tests
just test-integration
```

## Framework: Bats

This test suite uses [Bats](https://github.com/bats-core/bats-core), the industry-standard Bash testing framework. Bats provides:

- **TAP compliance** - Test Anything Protocol output for CI integration
- **BDD-style naming** - Human-readable test descriptions
- **Rich assertions** - Built-in assertion library (bats-assert)
- **Tag filtering** - Run subsets of tests by category

## Test Structure

```
tests/
├── bats/                          # Bats core framework (git submodule)
├── test_helper/
│   ├── bats-support/              # Formatting helpers (git submodule)
│   ├── bats-assert/               # Assertion library (git submodule)
│   ├── bats-file/                 # File assertions (git submodule)
│   ├── git-test-helpers.bash      # Git repo setup/teardown helpers
│   ├── threshold-helpers.bash     # Threshold calculation helpers
│   └── json-assertions.bash       # JSON validation helpers
├── unit/                          # Unit tests (*.bats)
│   ├── weighted-threshold.bats    # Weighted scoring logic tests
│   ├── incremental-tracking.bats  # Delete+recreate bug fix tests
│   ├── breakdown-formatting.bats  # Display formatting tests
│   └── test-helpers.bats          # Meta-tests for helper libraries
├── integration/                   # Integration tests (*.bats)
│   └── hook-contracts.bats        # Hook JSON schema validation
├── fixtures/                      # Static test data
│   ├── session-start.json
│   ├── stop.json
│   └── test-env.txt
└── manual/                        # Manual testing tools
    └── trip-threshold.sh          # Generate code to trip threshold
```

## Installation

### First-time setup (after cloning repository)

```bash
# Initialize git submodules
just setup-tests

# Or manually:
git submodule update --init --recursive
```

The test framework is self-contained - no global installation required.

## Running Tests

### Basic Commands

```bash
# Run all tests (unit + integration)
just test

# Run only unit tests
just test-unit

# Run only integration tests
just test-integration

# Run specific test file
just test-file weighted-threshold

# Show available tests and tags
just list
```

### Advanced Commands

```bash
# Run with TAP output (for CI)
just test-tap

# Filter by tag
just test-threshold      # Only threshold-related tests
just test-weighting      # Only weighting tests
just test-incremental    # Only incremental tracking tests
```

### Custom Filtering

```bash
# Run tests matching specific tags
tests/bats/bin/bats --filter-tags bug-fix tests/unit/*.bats

# Run multiple tag filters (OR logic)
tests/bats/bin/bats --filter-tags unit --filter-tags threshold tests/**/*.bats

# Exclude tests by tag
tests/bats/bin/bats --filter-tags '!slow' tests/**/*.bats
```

## Test Tags

Tests are organized with tags for flexible filtering:

| Tag | Purpose | Example Tests |
|-----|---------|---------------|
| `unit` | Unit tests | All tests in `tests/unit/` |
| `integration` | Integration tests | Hook contract validation |
| `threshold` | Threshold calculation | Scoring, penalties, limits |
| `weighting` | File weighting logic | New vs edited file multipliers |
| `incremental` | Incremental tracking | Delta accumulation, delete+recreate |
| `formatting` | Display formatting | Breakdown messages, percentages |
| `hooks` | Hook contracts | SessionStart, Stop validation |
| `bug-fix` | Bug fix verification | Specific regressions |

## Test Coverage

**Unit Tests (tests/unit/):**
- `weighted-threshold.bats` - 6 tests
  - Baseline scoring for new files
  - 1.3× multiplier for edited files
  - Scatter penalties (6-10 files, 11+ files)
  - Mixed new/edited files
  - Breakdown formatting

- `incremental-tracking.bats` - 3 tests
  - Delete+recreate bug fix
  - Modified content tracking
  - Multi-turn delta accumulation

- `breakdown-formatting.bats` - 7 tests
  - Basic percentage display
  - Different threshold limits
  - Scatter penalty display
  - Threshold exceeded (>100%)
  - Incremental mode display

- `test-helpers.bats` - 7 tests
  - Git repo creation
  - File generation helpers
  - JSON assertion validation

**Integration Tests (tests/integration/):**
- `hook-contracts.bats` - 3 tests
  - SessionStart hook schema
  - Stop hook schema
  - Environment variable capture

**Total: 26 test cases** (down from 27 after removing redundant smoke test)

## Writing New Tests

### Test File Template

Create a new `.bats` file in `tests/unit/` or `tests/integration/`:

```bash
# my-feature.bats - Description of what this file tests

# bats file_tags=unit,my-feature

# Load Bats libraries
load '../test_helper/bats-support/load'
load '../test_helper/bats-assert/load'

# Load custom helpers
load '../test_helper/git-test-helpers'
load '../test_helper/json-assertions'

# Setup: Run before each test
setup() {
  setup_git_test_repo
}

# Teardown: Run after each test
teardown() {
  cleanup_git_test_repo
}

# Test 1: BDD-style naming
# bats test_tags=tag1,tag2
@test "should do something specific when condition occurs" {
  # Arrange
  add_file_to_repo "test.txt" 50

  # Act
  run some_command

  # Assert
  assert_success
  assert_output "expected output"
}
```

### Helper Libraries

**Git Test Helpers** (`git-test-helpers.bash`):
```bash
setup_git_test_repo                    # Create temp git repo with baseline
cleanup_git_test_repo                  # Remove test repo
add_file_to_repo "file.txt" 50         # Create file with N lines
add_files_to_repo "f1.txt" 50 "f2.txt" 30  # Create multiple files
append_to_file "file.txt" 30           # Append N lines to existing file
commit_staged_changes "message"        # Commit with message
stage_and_capture_tree                 # Stage changes, update CURRENT_TREE
```

**JSON Assertions** (`json-assertions.bash`):
```bash
assert_json_field_exists "$json" ".field"           # Field presence
assert_json_field_equals "$json" ".field" "value"   # Exact match
assert_json_field_matches "$json" ".field" "regex"  # Regex match
assert_json_field_type "$json" ".field" "string"    # Type check
assert_json_fields_exist "$json" ".f1" ".f2"        # Multiple fields
assert_valid_json "$json_string"                    # Valid JSON
```

**Threshold Helpers** (`threshold-helpers.bash`):
```bash
threshold_data=$(calculate_full_threshold "$baseline" "$current")
score=$(calculate_and_extract "$baseline" "$current" ".weighted_score")
delta=$(calculate_incremental "$prev" "$curr" "$accumulated")
assert_threshold_score "$baseline" "$current" 200
assert_threshold_field "$baseline" "$current" "scatter_penalty" 70
```

### Bats Assertions

**Standard assertions from bats-assert:**
```bash
# Exit code
run command
assert_success          # Exit code 0
assert_failure          # Exit code != 0
assert_failure 127      # Specific exit code

# Output matching
assert_output "exact match"
assert_output --partial "substring"
assert_output --regexp "^pattern.*"

# Line-specific
assert_line 0 "first line"
assert_line --index 1 "second line"

# Comparisons
assert_equal "$actual" "$expected"
assert [ "$value" -eq 42 ]

# File operations (from bats-file)
assert_file_exist "/path/to/file"
assert_dir_exist "/path/to/dir"
```

## CI Integration

### GitHub Actions Example

```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          submodules: recursive
      - name: Run tests with TAP output
        run: just test-tap
```

### TAP Output

TAP (Test Anything Protocol) format is automatically used in CI environments:

```bash
# Detect CI and use TAP
just test-tap

# Or explicitly:
tests/bats/bin/bats --tap tests/**/*.bats
```

## Debugging Tests

### Verbose Output

```bash
# Show test names as they run
tests/bats/bin/bats --verbose-run tests/unit/my-test.bats

# Print all commands (like bash -x)
tests/bats/bin/bats --trace tests/unit/my-test.bats
```

### Run Single Test

```bash
# Run specific .bats file
tests/bats/bin/bats tests/unit/weighted-threshold.bats

# Or via justfile
just test-file weighted-threshold
```

### Inspect Failures

When a test fails, Bats shows:
- Test name and description
- Failed assertion with expected/actual values
- Line number in test file
- Command output (stdout/stderr)

Example failure output:
```
✗ should calculate baseline score for new files only
   (in test file tests/unit/weighted-threshold.bats, line 35)
     `assert_json_field_equals "$threshold_data" ".weighted_score" "200"' failed

   -- field: .weighted_score
   -- expected: 200
   -- actual: 150
   --
```

## Troubleshooting

### Submodules not initialized

**Error**: `tests/bats/bin/bats: No such file or directory`

**Fix**:
```bash
just setup-tests
# Or: git submodule update --init --recursive
```

### Tests fail unexpectedly

1. **Check git configuration**:
   ```bash
   git config user.name
   git config user.email
   ```

2. **Verify jq is installed**:
   ```bash
   which jq
   ```

3. **Run tests with trace**:
   ```bash
   tests/bats/bin/bats --trace tests/unit/failing-test.bats
   ```


## Best Practices

### Test Naming
- Use BDD-style: "should [expected behavior] when [condition]"
- Be specific: "should apply 1.3x multiplier to edited files"
- Not: "test_edited_files" (old convention)

### Test Organization
- One logical assertion per test
- Group related tests in same file
- Use tags for cross-cutting concerns

### Helper Usage
- Always use helpers instead of duplicating setup code
- Prefer `setup_git_test_repo` over manual `git init`
- Use `assert_json_field_*` for JSON validation

### Assertions
- Use bats-assert functions, not manual `[[ ]]` checks
- Prefer `assert_output` over `echo "$output" | grep`
- Use `assert_equal` for clear failure messages

## References

- [Bats Core Documentation](https://bats-core.readthedocs.io/)
- [bats-assert Library](https://github.com/bats-core/bats-assert)
- [bats-file Library](https://github.com/bats-core/bats-file)
- [Test Anything Protocol (TAP)](https://testanything.org/)

## Version Information

- **Bats Core**: 1.11.0
- **bats-support**: 0.3.0
- **bats-assert**: 2.1.0
- **bats-file**: 0.4.0

Check installed versions:
```bash
just version
```

## Migration Notes

This test suite was migrated from a custom Bash framework to Bats in November 2025. Key improvements:

- **Reduced duplication**: ~130 lines of duplicated patterns eliminated
- **Line count reduction**: 1,588 lines → ~900 lines (43% reduction)
- **Unified conventions**: 11 inconsistencies resolved
- **Industry standard**: TAP output, rich assertions
- **Better DX**: BDD naming, tag filtering, comprehensive helpers

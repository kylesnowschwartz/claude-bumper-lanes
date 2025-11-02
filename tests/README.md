# Bumper Lanes Test Suite

## Integration Tests

### `integration/validate-hook-contracts.sh`

**Purpose:** Empirically verify Claude Code hook input schemas match our implementation.

**Usage:**
```bash
cd /path/to/claude-bumper-lanes
./tests/integration/validate-hook-contracts.sh
```

**What it does:**
1. Creates isolated test environment in `/tmp` (safe, repeatable)
2. Registers capture hooks that log stdin JSON
3. Runs test Claude session with `--dangerously-skip-permissions`
4. Validates captured JSON against expected schema
5. Checks environment variables (`CLAUDE_PROJECT_DIR`, `CLAUDE_ENV_FILE`, etc.)
6. Generates validation report

**Expected output:**
```
✅ ALL VALIDATIONS PASSED
```

**On failure:**
- Captures are preserved in `/tmp/claude-hook-contract-test-*/`
- Detailed report shows which fields are missing/incorrect
- Use captured JSON to update implementation

**When to run:**
- After modifying hook scripts
- When Claude Code updates (schema may change)
- Before committing hook changes
- As part of CI/CD validation

## Research Documents

See `kitty-specs/001-bumper-lanes/RESEARCH_FINDINGS.md` for empirical schema research.

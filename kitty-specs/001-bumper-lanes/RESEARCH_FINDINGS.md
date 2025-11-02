# Claude Code Hook Input Schema - Empirical Research Results
**Date:** 2025-11-02
**Method:** Captured actual JSON sent to hooks during test session

## Actual Hook Input Schemas (Verified)

### SessionStart Hook

```json
{
  "session_id": "a135d3b1-0387-4c0e-9783-7af541189238",
  "transcript_path": "/Users/kyle/.claude/projects/-private-tmp-claude-hook-schema-test/a135d3b1-0387-4c0e-9783-7af541189238.jsonl",
  "cwd": "/private/tmp/claude-hook-schema-test",
  "hook_event_name": "SessionStart",
  "source": "startup"
}
```

**Fields:**
- `session_id` (string): UUID for conversation
- `transcript_path` (string): Path to JSONL transcript file
- `cwd` (string): Current working directory (project root)
- `hook_event_name` (string): Always "SessionStart"
- `source` (string): "startup", "resume", "clear", or "compact"

### Stop Hook

```json
{
  "session_id": "a135d3b1-0387-4c0e-9783-7af541189238",
  "transcript_path": "/Users/kyle/.claude/projects/-private-tmp-claude-hook-schema-test/a135d3b1-0387-4c0e-9783-7af541189238.jsonl",
  "cwd": "/private/tmp/claude-hook-schema-test",
  "permission_mode": "bypassPermissions",
  "hook_event_name": "Stop",
  "stop_hook_active": false
}
```

**Fields:**
- `session_id` (string): UUID for conversation
- `transcript_path` (string): Path to JSONL transcript file
- `cwd` (string): Current working directory (project root)
- `permission_mode` (string): Current permission mode
- `hook_event_name` (string): Always "Stop"
- `stop_hook_active` (boolean): True if Stop hook already active (prevent loops)

## Environment Variables Available to Hooks

```bash
CLAUDECODE=1
CLAUDE_CODE_ENTRYPOINT=cli
CLAUDE_PROJECT_DIR=/private/tmp/claude-hook-schema-test
CLAUDE_ENV_FILE=/Users/kyle/.claude/session-env/{session_id}/hook-0.sh
CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR=1
PWD=/tmp/claude-hook-schema-test
```

**Key Variables:**
- `CLAUDE_PROJECT_DIR`: Absolute path to project root
- `CLAUDE_ENV_FILE`: Path to write env vars for SessionStart hooks
- `CLAUDECODE=1`: Indicator that code is running inside Claude Code
- `PWD`: Hook process is already cd'd to project directory

## Key Findings

### ✅ Confirmed Correct
1. `session_id` uses snake_case (not camelCase `sessionId`)
2. Hook event name field is `hook_event_name` (not `hook_name`)
3. Hooks are spawned with `PWD` already set to project directory

### ❌ Documentation Gaps
1. Official docs don't mention `cwd` field (but it exists!)
2. Docs show `permission_mode` field missing from SessionStart (correct)
3. Docs show `source` field missing from Stop (correct)

### 🔧 Implementation Corrections Needed
1. Change `.sessionId` → `.session_id`
2. Change `.working_directory` → `.cwd`
3. Change `.hook_name` → `.hook_event_name`
4. Remove `cd "$working_dir"` - hooks already run in correct directory

## Test Command Used

```bash
claude -p "Echo test 123" \
  --model haiku \
  --settings /tmp/claude-hook-schema-test/test-settings.json \
  --dangerously-skip-permissions
```

## Verification Method

Created capture hooks that log stdin JSON to timestamped files:
- `/tmp/claude-hook-schema-test/capture-session-start.sh`
- `/tmp/claude-hook-schema-test/capture-stop.sh`

Registered via test settings file, ran test session, parsed captured JSON.

## Conclusion

**The official documentation is incomplete.** The `cwd` field exists but isn't documented. Always verify hook schemas empirically with capture scripts rather than trusting docs alone.

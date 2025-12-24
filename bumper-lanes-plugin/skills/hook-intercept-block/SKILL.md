---
name: hook-intercept-block
description: >
  This skill should be used when implementing slash commands that execute without
  Claude API calls. Use when: adding a new /bumper-* command, understanding why
  commands return "block" responses, debugging UserPromptSubmit hooks, or learning
  the pattern for instant command execution. Keywords: UserPromptSubmit, block decision,
  hook response, slash command implementation.
---

# Hook-Intercept-Block Pattern

Pattern for implementing slash commands that execute entirely in the hook handler,
bypassing the Claude API call entirely.

## Why Use This Pattern

- **No API cost**: Commands execute in Go, no Claude API call
- **Faster**: Direct execution vs markdown parsing + API round trip
- **Deterministic**: No model variance - same input, same output

## How It Works

```
User types: /bumper-reset
    ↓
UserPromptSubmit hook fires
    ↓
prompt_handler.go matches regex: ^/(?:claude-bumper-lanes:)?bumper-reset\s*$
    ↓
handleReset() executes Go logic
    ↓
Returns JSON to stdout: {"decision":"block","reason":"Baseline reset. Score: 0/400"}
    ↓
Claude Code shows "reason" to user, skips API call
```

## The Confusing Naming

Claude Code's hook response API uses counterintuitive terminology:

| Response | What It Actually Means |
|----------|----------------------|
| `decision: "block"` | "I handled this, don't call Claude API" (NOT "blocked/rejected") |
| `decision: "continue"` | "Let it through to Claude API" |
| `reason: "..."` | Message shown to user (only with "block") |

**Key insight**: `block` = "handled and done", not "rejected". The command succeeded.

## Implementation Components

1. **Hook config** (`hooks.json`): Routes UserPromptSubmit to handler binary
2. **Handler** (`internal/hooks/prompt_handler.go`): Regex matching + dispatch
3. **Command stubs** (`commands/*.md`): MUST exist for `/help` discovery (body ignored)

## Adding a New Command

### Step 1: Add Regex Pattern

In `prompt_handler.go`:

```go
var newCmdPattern = regexp.MustCompile(`^/(?:claude-bumper-lanes:)?bumper-foo\s*(.*)$`)
```

The `(?:claude-bumper-lanes:)?` makes the plugin namespace optional.

### Step 2: Add Dispatch

In `HandlePrompt()`:

```go
if m := newCmdPattern.FindStringSubmatch(prompt); m != nil {
    return handleFoo(sessionID, strings.TrimSpace(m[1]))
}
```

### Step 3: Implement Handler

Use the helper functions for DRY session management:

```go
func handleFoo(sessionID, args string) int {
    sess := loadSessionOrBlock(sessionID)
    if sess == nil {
        return 0
    }

    // ... your logic here ...

    if !saveOrBlock(sess) {
        return 0
    }

    blockPrompt("Success message")
    return 0
}
```

### Step 4: Create Command Stub

Create `commands/bumper-foo.md`:

```markdown
---
description: Does the foo thing
argument-hint: <optional-args>
---

This command is handled by the hook system.
```

The markdown body is ignored - the hook handles everything. The file MUST exist
for the command to appear in `/help`.

### Step 5: Rebuild

```bash
just build-bumper-lanes
```

## Helper Functions

Two helpers reduce boilerplate:

### loadSessionOrBlock

```go
func loadSessionOrBlock(sessionID string) *state.SessionState
```

Returns session state or nil. If nil, error already shown to user via `blockPrompt()`.

### saveOrBlock

```go
func saveOrBlock(sess *state.SessionState) bool
```

Returns true on success. If false, error already shown to user via `blockPrompt()`.

## JSON Response Format

The `UserPromptResponse` struct:

```go
type UserPromptResponse struct {
    Decision string `json:"decision,omitempty"`
    Reason   string `json:"reason,omitempty"`
}
```

Output via `blockPrompt()`:

```go
func blockPrompt(reason string) {
    resp := UserPromptResponse{
        Decision: "block",
        Reason:   reason,
    }
    out, _ := json.Marshal(resp)
    fmt.Println(string(out))
}
```

## Existing Commands Using This Pattern

All bumper-lanes slash commands use hook-intercept-block:

| Command | Handler | Purpose |
|---------|---------|---------|
| `/bumper-reset` | `handleReset()` | Capture new baseline, reset score |
| `/bumper-pause` | `handlePause()` | Disable enforcement |
| `/bumper-resume` | `handleResume()` | Re-enable enforcement |
| `/bumper-view` | `handleView()` | Set/show visualization mode |
| `/bumper-config` | `handleConfig()` | Show/set threshold |

## Debugging Tips

1. **Command not recognized**: Check regex pattern matches user input exactly
2. **No output shown**: Ensure `blockPrompt()` is called and JSON printed to stdout
3. **Command not in /help**: Verify `commands/*.md` stub file exists
4. **Binary not updated**: Run `just build-bumper-lanes` after changes

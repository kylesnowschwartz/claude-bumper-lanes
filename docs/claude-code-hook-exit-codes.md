# Claude Code Hook Exit Codes

## Exit Code Meanings

```
 | Code   | Name    | Effect                                     |
 | ------ | ------  | --------                                   |
 | 0      | Success | Operation allowed, continue                |
 | 1      | Warning | Operation allowed, show warning to user    |
 | 2      | Block   | Operation blocked, send feedback to Claude |
 | Other  | Warning | Treated same as exit 1                     |
```

## Output Routing by Hook Type and Exit Code

### UserPromptSubmit

```
| Exit   | STDOUT →       | STDERR →   | Operation   | Prompt State   |
| ------ | ----------     | ---------- | ----------- | -------------- |
| 0      | Claude context | None       | Allowed     | Processed      |
| 1      | None           | User       | Allowed     | Processed      |
| 2      | None           | User       | **BLOCKED** | Erased         |
```

### PreToolUse

```
 | Exit   | STDOUT →        | STDERR →   | Operation      | Tool State   |
 | ------ | ----------      | ---------- | -----------    | ------------ |
 | 0      | User transcript | None       | Allowed        | Runs         |
 | 1      | User transcript | User       | Ask permission | User decides |
 | 2      | User transcript | **Claude** | **BLOCKED**    | Denied       |
```

Notes:
- Exit 0 with JSON `permissionDecision: "deny"` also blocks tool
- Exit 0 with JSON `permissionDecision: "ask"` asks user permission
- Modern hooks use JSON output on STDOUT with exit 0 instead of exit codes

### PostToolUse

```
 | Exit   | STDOUT →        | STDERR →   | Operation   | Effect                  |
 | ------ | ----------      | ---------- | ----------- | --------                |
 | 0      | User transcript | None       | Complete    | Continue                |
 | 1      | User transcript | User       | Complete    | Show warning            |
 | 2      | User transcript | **Claude** | Complete    | Send feedback to Claude |
```

Note: Tool has already run. This hook provides feedback only.

### Stop / SubagentStop

```
 | Exit   | STDOUT →        | STDERR →   | Agent State   | Effect                                 |
 | ------ | ----------      | ---------- | ------------- | --------                               |
 | 0      | User transcript | None       | **STOPS**     | Agent stops normally                   |
 | 1      | User transcript | User       | **STOPS**     | Agent stops with warning               |
 | 2      | None            | **Claude** | **CONTINUES** | Stop blocked, agent forced to continue |
```

Critical: Exit 2 **blocks stoppage**. Agent cannot finish turn. STDERR message goes to Claude.

### SessionStart

```
 | Exit   | STDOUT →       | STDERR →   | Operation   | Effect                      |
 | ------ | ----------     | ---------- | ----------- | --------                    |
 | 0      | Claude context | None       | Allowed     | Session starts with context |
 | 1      | None           | User       | Allowed     | Session starts with warning |
 | 2      | None           | User       | Allowed     | Treated as exit 1           |
```

### SessionEnd

```
 | Exit     | STDOUT →   | STDERR →   | Operation   | Effect                     |
 | ------   | ---------- | ---------- | ----------- | --------                   |
 | Always 0 | Debug log  | Debug log  | Complete    | Cleanup only, cannot block |
```

### Notification

```
 | Exit   | STDOUT →   | STDERR →   | Operation   | Effect            |
 | ------ | ---------- | ---------- | ----------- | --------          |
 | 0      | Debug log  | None       | Continue    | Logged only       |
 | 1      | Debug log  | User       | Continue    | Warning logged    |
 | 2      | Debug log  | User       | Continue    | Treated as exit 1 |
```

### PreCompact

```
 | Exit   | STDOUT →        | STDERR →   | Operation   | Effect            |
 | ------ | ----------      | ---------- | ----------- | --------          |
 | 0      | User transcript | None       | Allowed     | Continue          |
 | 1      | User transcript | User       | Allowed     | Show warning      |
 | 2      | User transcript | User       | Allowed     | Treated as exit 1 |
```

## References

Web: https://docs.claude.com/en/docs/claude-code/hooks

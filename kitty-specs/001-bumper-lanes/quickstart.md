# Quickstart: Bumper Lanes Plugin

**Target Audience**: Developers who want to prevent uncontrolled code changes during AI agent sessions

**Time to Complete**: 5-10 minutes

---

## What is Bumper Lanes?

Bumper Lanes is a Claude Code plugin that tracks how many changes your AI agent makes via git, and blocks execution once a threshold is exceeded. This forces you to review code before the agent continues, promoting disciplined "vibe coding."

**Key Features**:
- 🚦 Automatic threshold enforcement (default: 300 lines changed)
- 📊 Real-time diff statistics tracking
- 🔄 Explicit consent mechanism via `/bumper-reset` command
- 🔒 Non-destructive (preserves git index state)
- ⚡ Low overhead (<500ms per agent stop)

---

## Prerequisites

- Claude Code installed and configured
- Git 2.x+ installed
- Bash 4.0+ (macOS/Linux)
- `jq` command-line JSON processor

**Check your prerequisites**:
```bash
# Check Claude Code
claude --version

# Check git
git --version

# Check bash
bash --version

# Install jq if missing
## macOS
brew install jq

## Linux
sudo apt-get install jq  # Debian/Ubuntu
sudo yum install jq      # RHEL/CentOS
```

---

## Installation

### Method 1: From Marketplace (Recommended)

Once the plugin is published to a marketplace:

```bash
# In Claude Code session
/plugin marketplace add kylesnowschwartz/claude-plugins

# Install the plugin
/plugin install claude-bumper-lanes@kylesnowschwartz

# Verify installation
/plugin list
```

### Method 2: Local Development/Testing

For plugin developers or testing unreleased versions:

**Step 1: Clone the repository**
```bash
git clone https://github.com/kylesnowschwartz/claude-bumper-lanes.git
cd claude-bumper-lanes
```

**Step 2: Validate plugin structure**
```bash
claude plugin validate .
```

**Step 3: Create a local marketplace**

Create `.claude-plugin/marketplace.json`:
```json
{
  "name": "local-dev",
  "owner": {
    "name": "Your Name",
    "email": "you@example.com"
  },
  "plugins": [
    {
      "name": "claude-bumper-lanes",
      "version": "1.0.0",
      "source": "./",
      "description": "Git diff threshold enforcement"
    }
  ]
}
```

**Step 4: Add local marketplace and install**
```bash
# In Claude Code session, from plugin directory
/plugin marketplace add .

# Install from local marketplace
/plugin install claude-bumper-lanes@local-dev
```

### Method 3: Repository Configuration (Team Installation)

Add to your project's `.claude/settings.json`:

```json
{
  "plugins": {
    "marketplaces": [
      "kylesnowschwartz/claude-plugins"
    ],
    "installed": [
      "claude-bumper-lanes@kylesnowschwartz"
    ]
  }
}
```

Plugins configured this way install automatically when team members open the project in Claude Code.

### Verify Installation

```bash
# In Claude Code session
/help

# Look for /bumper-reset command in the list
```

---

## Basic Usage

### 1. Start a Claude Code Session

```bash
cd /path/to/your/project
claude-code
```

**What happens**: The plugin captures a baseline snapshot of your current git state.

### 2. Make Changes with Claude

```
You: Add a new user authentication feature
Claude: [makes code changes...]
```

**What happens**: Each time Claude stops (after generating code), the plugin:
1. Computes diff stats (lines added/deleted) since baseline
2. Checks if total lines changed exceeds threshold (default: 300)
3. If under threshold: Claude continues normally
4. If over threshold: Claude is blocked with a message

### 3. Hit the Threshold

When threshold is exceeded, you'll see:

```
⚠ Diff threshold exceeded: 430/300 lines changed (143%).

Changes:
  8 files changed, 287 insertions(+), 143 deletions(-)

Files modified:
  src/auth/login.rs       (+45, -12)
  src/auth/session.rs     (+89, -23)
  src/models/user.rs      (+34, -8)
  ...

Review your changes and run /bumper-reset to continue.
```

**Claude is now blocked** and cannot generate more code until you consent.

### 4. Review and Reset

Review the changes:
```bash
git status
git diff
```

If changes look good, reset the baseline:
```
You: /bumper-reset
```

**What happens**:
- Current state becomes new baseline
- Diff counter resets to zero
- Claude can continue with fresh threshold budget

### 5. Continue Coding

```
You: Now add password reset functionality
Claude: [continues with new baseline...]
```

---

## Configuration

### Default Configuration

The plugin ships with sensible defaults:
- **Threshold**: 300 lines changed (additions + deletions)
- **Metric**: Simple line count
- **Block subagents**: Yes
- **Enabled**: Yes

### Custom Configuration

#### Repository-Specific (Highest Priority)

Create `.git/bumper-config.json`:

```json
{
  "metric": "simple-line-count",
  "limit": 500,
  "enabled": true,
  "block_subagents": true
}
```

#### User-Level (Medium Priority)

Create `~/.config/claude-code/bumper-lanes.json`:

```json
{
  "metric": "simple-line-count",
  "limit": 200,
  "enabled": true,
  "block_subagents": false
}
```

#### Plugin Defaults (Lowest Priority)

Located at `~/.config/claude-code/plugins/claude-bumper-lanes/config/bumper-lanes.json`

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `metric` | string | `"simple-line-count"` | Threshold calculation method |
| `limit` | integer | `300` | Maximum total lines changed |
| `enabled` | boolean | `true` | Enable/disable plugin |
| `block_subagents` | boolean | `true` | Block subagent stops too |

**Precedence**: Repository > User > Plugin Defaults

---

## Slash Commands

### `/bumper-reset`

Resets the baseline to current git state and allows Claude to continue.

**Usage**:
```
/bumper-reset
```

**When to use**:
- After reviewing code and deciding changes are acceptable
- When threshold is exceeded and you want to continue
- To checkpoint progress before large refactoring

**Output**:
```
✓ Baseline reset complete.

Previous baseline: 3a4b5c6 (captured 2025-11-02 20:15:00)
New baseline: 1f2e3d4 (captured 2025-11-02 20:45:30)

Changes accepted: 8 files, 287 insertions(+), 143 deletions(-) [430 lines total]

You now have a fresh diff budget of 300 lines. Continue coding!
```

### `/bumper-status` (Optional Feature)

Shows current diff statistics and remaining threshold budget.

**Usage**:
```
/bumper-status
```

**Output**:
```
Bumper Lanes Status:

Baseline: 3a4b5c6d (captured 2025-11-02 20:15:00)
Current:  1f2e3d4c

Changes:
  5 files changed, 127 insertions(+), 48 deletions(-)
  Total lines changed: 175

Threshold: 175/300 lines (58%)
Remaining budget: 125 lines
```

---

## Troubleshooting

### Plugin Not Working

**Check hooks are registered**:
```bash
cd ~/.config/claude-code/plugins/claude-bumper-lanes
cat .claude-plugin/plugin.json
```

**Check scripts are executable**:
```bash
ls -l hooks/scripts/*.sh
# Should show -rwxr-xr-x permissions
```

**Enable plugin debug logging**:
```bash
export BUMPER_DEBUG=1
claude-code
```

### Baseline Not Captured

**Symptom**: `/bumper-reset` says "No active session found"

**Fix**:
1. Exit Claude Code session
2. Start new session (triggers SessionStart hook)
3. Check `.git/bumper-checkpoints/session-*` file exists

### Threshold Not Blocking

**Symptom**: Large changes made but no block message

**Debug**:
```bash
# Check session state
cat .git/bumper-checkpoints/session-$(pgrep -f claude-code | head -1)

# Check hook is firing
tail -f ~/.config/claude-code/logs/hooks.log
```

### Git Errors

**Symptom**: Hook errors related to git commands

**Common Causes**:
- Not in a git repository (plugin auto-disables)
- Git repo corrupted (run `git fsck`)
- Permissions issue (check `.git/` is writable)

### Manual Cleanup

**Remove orphaned state files**:
```bash
# List old sessions (PIDs no longer running)
ls .git/bumper-checkpoints/session-*

# Remove manually
rm .git/bumper-checkpoints/session-12345
```

---

## FAQ

**Q: What happens if I manually edit files during a Claude session?**
A: The plugin tracks *all* changes in your working tree, not just agent changes. Manual edits count toward the threshold.

**Q: Can I exclude certain files from threshold calculation?**
A: Not in v1. Future versions may support `.bumperignore` patterns.

**Q: Does this work with git worktrees?**
A: Yes! Each worktree has its own `.git/` directory, so plugin tracks per-worktree.

**Q: What happens on git branch switch during session?**
A: Baseline tree objects persist in `.git/objects/` regardless of branch. Diff calculation continues normally.

**Q: Can I use this with multiple concurrent Claude sessions?**
A: Yes! Plugin uses PID-based state isolation (`.git/bumper-checkpoints/session-$$`).

**Q: Does plugin create git commits?**
A: No. Plugin uses git's internal tree objects without creating commits.

**Q: What's the performance overhead?**
A: Typically <500ms per agent stop for repos with <100k files. Larger repos may see 1-2s overhead.

---

## Next Steps

1. **Read the implementation guide** to understand hook internals
2. **Review data model** to see state management design
3. **Run tests** to validate plugin behavior
4. **Customize threshold** for your workflow
5. **Give feedback** on GitHub issues

---

## Getting Help

- **Documentation**: See `README.md` in plugin directory
- **Issues**: https://github.com/kylesnowschwartz/claude-bumper-lanes/issues
- **Discussions**: https://github.com/kylesnowschwartz/claude-bumper-lanes/discussions

---

**Happy disciplined vibe coding! 🚦**

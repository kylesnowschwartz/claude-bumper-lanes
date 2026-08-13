package hooks

import (
	"fmt"
	"os"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/events"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/statusline"
)

// SessionStart handles the SessionStart hook event.
// It captures the baseline tree and initializes session state.
// Returns exit code: 0 = success, 1 = warning (shows stderr to user).
func SessionStart(input *hookio.Input) int {
	// Initialize logger for this session
	log := logging.New(input.SessionID, "session_start")

	// Check if this is a git repository
	if !git.IsRepo() {
		return 0 // Fail open - not a git repo
	}

	// Compaction and resume reuse the session id, so re-baselining here would
	// silently refill the budget mid-task. Preserve the existing state and
	// re-inject the budget into Claude's context (which compaction just wiped).
	if input.Source == "compact" || input.Source == "resume" {
		if sess, err := state.Load(input.SessionID); err == nil {
			return emitBudgetRecap(sess, input.Source)
		}
		// No existing state for this session id - fall through to fresh baseline
	}

	// Capture baseline tree
	baselineTree, err := git.CaptureTree()
	if err != nil {
		log.Warn("failed to capture baseline tree: %v (failing open)", err)
		return 0 // Fail open
	}

	// Get current branch for staleness detection
	baselineBranch := git.CurrentBranch()

	cfg := loadConfig(log)
	threshold := cfg.Threshold

	// Create and save session state
	sess, err := state.New(input.SessionID, baselineTree, baselineBranch, threshold)
	if err != nil {
		log.Warn("failed to create session state: %v (failing open)", err)
		return 0 // Fail open
	}
	// Anchor the baseline at today's HEAD so later pulls/rebases can be
	// forgiven (maybeRebaseBaseline).
	sess.BaselineHead = git.HeadCommit()
	// Stamp the trip policy while we're in a hook process, where plugin
	// userConfig env vars are visible (Bash-invoked CLI commands are not).
	sess.Policy = reviewPolicy(cfg)

	if err := sess.Save(); err != nil {
		log.Warn("failed to save session state: %v (failing open)", err)
		return 0 // Fail open
	}

	source := input.Source
	if source == "" {
		source = "startup"
	}
	if err := events.Append(events.Entry{SessionID: input.SessionID, Event: events.SessionStart, Score: 0, Limit: threshold, Cause: source}); err != nil {
		log.Warn("failed to append event: %v", err)
	}

	// Collect warnings to show user (exit 1 with stderr shows warnings)
	var warnings []string

	// Check for excessive checkpoint accumulation
	if warning := state.CheckpointCountWarning(); warning != "" {
		warnings = append(warnings, warning)
	}

	// Fresh status line installation is opt-in (statusline_auto_setup: true)
	// because it rewrites ~/.claude/settings.json, a user-global file.
	// Repairing an already-installed bumper-lanes wrapper or binary path
	// always runs: the user opted in by installing, and a plugin update
	// that moves the binary would otherwise silently break their status line.
	if msg := statusline.EnsureInstalled(log, cfg.StatuslineAutoSetup); msg != "" {
		warnings = append(warnings, msg)
	}

	// Show all warnings via stderr + exit 1
	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, w)
		}
		return 1 // Exit 1 shows stderr to user
	}

	return 0
}

// emitBudgetRecap injects the preserved budget state into Claude's context.
// Silent (but still preserving state) when enforcement is paused or disabled.
func emitBudgetRecap(sess *state.SessionState, source string) int {
	if sess.Paused || sess.ThresholdLimit == 0 {
		return 0
	}
	msg := fmt.Sprintf(
		"bumper-lanes: %s, preserved across %s. Incremental-review contract active: plan work that fits the remaining budget, or ask before expanding scope.",
		budgetLine(sess.Score, sess.ThresholdLimit), source)
	if err := hookio.WriteContext("SessionStart", msg); err != nil {
		logging.New(sess.SessionID, "session_start").Warn("failed to write budget recap context: %v (failing open)", err)
	}
	return 0
}

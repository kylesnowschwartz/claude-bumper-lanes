// Package events appends lifecycle events to an append-only JSONL log at
// {git-dir}/bumper-checkpoints/events.jsonl. The log is a measuring
// instrument (analyzed ad hoc with jq), not a display surface: it answers
// questions like "how large is a typical reviewed increment" and "how often
// does each reset cause fire".
package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
)

// Event names.
const (
	SessionStart = "session_start"
	Trip         = "trip"
	Reset        = "reset"
	Pause        = "pause"
	Resume       = "resume"
	Tripwire     = "tripwire"
	// Rebase marks the baseline advancing over commits that landed since it
	// was captured (pull/rebase/merge/external commit). Not a reset: the
	// score continues, minus the upstream churn. Its score field is the
	// score before the rebase.
	Rebase = "rebase"
)

// Reset causes. A reset entry's score is the score at reset time, which is
// the size of the increment that just ended.
const (
	CauseManual         = "manual"
	CauseCommit         = "commit"
	CauseVerifiedCommit = "verified-commit"
	CauseCleanTree      = "clean-tree"
	CauseBranch         = "branch"
	// CauseReview marks a self-review clear (on_trip: review): the agent
	// reviewed the increment and cleared the breaker itself.
	CauseReview = "review"
	// CauseUpstream marks a baseline rebase over commits the session did
	// not make through its own commit flow (pull, rebase, merge).
	CauseUpstream = "upstream"
)

// Entry is one logged lifecycle event.
type Entry struct {
	TS        string `json:"ts"`
	SessionID string `json:"session_id"`
	Event     string `json:"event"`
	Score     int    `json:"score"`
	Limit     int    `json:"limit"`
	Cause     string `json:"cause,omitempty"`
	Tripwire  string `json:"tripwire,omitempty"`
}

// logPath returns the events file path, or "" outside a git repo.
func logPath() string {
	gitDir, err := config.GetGitDir()
	if err != nil {
		return ""
	}
	return filepath.Join(gitDir, "bumper-checkpoints", "events.jsonl")
}

// Append writes one event line, stamping the timestamp. Best-effort:
// errors are returned for optional logging but must never block the
// operation being recorded (fail-open, like every other bumper-lanes path).
func Append(e Entry) error {
	path := logPath()
	if path == "" {
		return nil // not a git repo - nothing to record against
	}
	e.TS = time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

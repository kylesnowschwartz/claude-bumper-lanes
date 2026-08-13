// Package state provides session state management for bumper-lanes.
// State is persisted in {git-dir}/bumper-checkpoints/session-{session_id}.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SessionState represents the persisted state for a bumper-lanes session.
type SessionState struct {
	SessionID      string `json:"session_id"`
	BaselineTree   string `json:"baseline_tree"`
	BaselineBranch string `json:"baseline_branch,omitempty"`
	Score          int    `json:"score"` // Current score (fresh calculation from baseline)
	CreatedAt      string `json:"created_at"`
	ThresholdLimit int    `json:"threshold_limit"`
	RepoPath       string `json:"repo_path"`
	StopTriggered  bool   `json:"stop_triggered"`
	Paused         bool   `json:"paused,omitempty"`
	// NetLines is the additions-minus-deletions of the current increment,
	// cached whenever the baseline diff is computed. Negative means the
	// tree shrank; the statusline shows that in green.
	NetLines int `json:"net_lines,omitempty"`

	// BaselineHead is the HEAD commit SHA at the moment the baseline was
	// captured. When HEAD later moves (pull, rebase, merge, external commit),
	// the baseline is rebased over the old-HEAD→new-HEAD delta so upstream
	// changes are not charged against the review budget. Empty on state
	// written by older versions; adopted lazily on the next score calculation.
	BaselineHead string `json:"baseline_head,omitempty"`

	// HeadBeforeCommit is the HEAD commit SHA recorded by PreToolUse when a
	// commit-shaped Bash command is about to run ("none" on an unborn branch).
	// PostToolUse treats a moved HEAD as the evidence that the commit
	// succeeded. Empty means no commit is pending.
	HeadBeforeCommit string `json:"head_before_commit,omitempty"`

	// Tripwires are the zero-threshold hits detected in the current
	// increment (tripwire file paths, "pattern (file)" for added lines).
	// Cleared on baseline reset.
	Tripwires []string `json:"tripwires,omitempty"`

	// AutoReviews counts consecutive self-review clears (on_trip: review)
	// since the last human-visible reset. At 1 the next trip escalates to
	// the human packet, so the agent gets one self-review per human
	// touchpoint. Zeroed by manual reset and commit auto-reset.
	AutoReviews int `json:"auto_reviews,omitempty"`
}

// ErrNoSession is returned when the session state file doesn't exist.
var ErrNoSession = errors.New("no session state found")

// GetCheckpointDir returns the absolute path to the checkpoint directory.
// Handles git worktrees where .git is a file, not a directory.
func GetCheckpointDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--absolute-git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	gitDir := strings.TrimSpace(string(output))
	return filepath.Join(gitDir, "bumper-checkpoints"), nil
}

// GetRepoPath returns the repository root path.
func GetRepoPath() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// stateFilePath returns the path to the state file for a session.
func stateFilePath(sessionID string) (string, error) {
	checkpointDir, err := GetCheckpointDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(checkpointDir, "session-"+sessionID), nil
}

// Load reads session state from disk.
// Returns ErrNoSession if the state file doesn't exist.
func Load(sessionID string) (*SessionState, error) {
	path, err := stateFilePath(sessionID)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSession
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	return &state, nil
}

// Save writes session state to disk atomically.
// Uses temp file + rename to prevent race conditions.
func (s *SessionState) Save() error {
	path, err := stateFilePath(s.SessionID)
	if err != nil {
		return err
	}

	// Ensure checkpoint directory exists
	checkpointDir := filepath.Dir(path)
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return fmt.Errorf("creating checkpoint dir: %w", err)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	// Atomic write: temp file + rename
	tempFile, err := os.CreateTemp(checkpointDir, "session-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// New creates a new SessionState with initial values.
func New(sessionID, baselineTree, baselineBranch string, thresholdLimit int) (*SessionState, error) {
	repoPath, err := GetRepoPath()
	if err != nil {
		repoPath = ""
	}

	return &SessionState{
		SessionID:      sessionID,
		BaselineTree:   baselineTree,
		BaselineBranch: baselineBranch,
		Score:          0,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		ThresholdLimit: thresholdLimit,
		RepoPath:       repoPath,
		StopTriggered:  false,
		Paused:         false,
	}, nil
}

// LoadLatest reads the most recently modified session state in the
// checkpoint directory. Used by CLI commands that run without a session id
// (e.g. `bumper-lanes budget` invoked from Claude's Bash tool).
func LoadLatest() (*SessionState, error) {
	checkpointDir, err := GetCheckpointDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		return nil, ErrNoSession
	}

	var latestID string
	var latestMod time.Time
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "session-") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestMod) {
			latestMod = info.ModTime()
			latestID = strings.TrimPrefix(name, "session-")
		}
	}
	if latestID == "" {
		return nil, ErrNoSession
	}
	return Load(latestID)
}

// Delete removes the session state file.
func Delete(sessionID string) error {
	path, err := stateFilePath(sessionID)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// SetStopTriggered updates the stop_triggered flag.
func (s *SessionState) SetStopTriggered(triggered bool) {
	s.StopTriggered = triggered
}

// SetPaused updates the paused flag.
func (s *SessionState) SetPaused(paused bool) {
	s.Paused = paused
}

// SetScore updates the current score (fresh calculation from baseline).
func (s *SessionState) SetScore(score int) {
	s.Score = score
}

// ResetBaseline resets the baseline to a new tree SHA, anchored at newHead.
// Clears score, net lines, stop_triggered, tripwires, and the auto-review
// counter (every reset path except review-clear is human-visible; review-clear
// restores its own incremented counter after calling this).
func (s *SessionState) ResetBaseline(newTree, newBranch, newHead string) {
	s.BaselineTree = newTree
	s.BaselineHead = newHead
	s.Score = 0
	s.NetLines = 0
	s.StopTriggered = false
	s.Tripwires = nil
	s.AutoReviews = 0
	if newBranch != "" {
		s.BaselineBranch = newBranch
	}
}

// AddTripwires records tripwire hits, returning only the ones not already
// known so callers warn once per hit per increment.
func (s *SessionState) AddTripwires(hits []string) []string {
	known := make(map[string]bool, len(s.Tripwires))
	for _, t := range s.Tripwires {
		known[t] = true
	}
	var fresh []string
	for _, h := range hits {
		if !known[h] {
			known[h] = true
			s.Tripwires = append(s.Tripwires, h)
			fresh = append(fresh, h)
		}
	}
	return fresh
}

// CheckpointWarningThreshold is the number of checkpoint files that triggers a warning.
const CheckpointWarningThreshold = 100

// CountCheckpoints returns the number of session checkpoint files.
// Returns 0 on any error (fail-open).
func CountCheckpoints() int {
	checkpointDir, err := GetCheckpointDir()
	if err != nil {
		return 0
	}

	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "session-") && !strings.HasSuffix(name, ".tmp") {
			count++
		}
	}
	return count
}

// CheckpointCountWarning returns a warning message if checkpoint count exceeds threshold.
// Returns empty string if count is acceptable.
func CheckpointCountWarning() string {
	count := CountCheckpoints()
	if count >= CheckpointWarningThreshold {
		checkpointDir, _ := GetCheckpointDir()
		return fmt.Sprintf("[bumper-lanes] %d checkpoint files accumulated. Run: rm -rf %q", count, checkpointDir)
	}
	return ""
}

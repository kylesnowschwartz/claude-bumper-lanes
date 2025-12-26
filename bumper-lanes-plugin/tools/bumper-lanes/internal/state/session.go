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
	ViewMode       string `json:"view_mode,omitempty"`
	ViewOpts       string `json:"view_opts,omitempty"` // Additional flags like "--width 100"
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

// ResetBaseline resets the baseline to a new tree SHA.
// Clears score and stop_triggered.
func (s *SessionState) ResetBaseline(newTree, newBranch string) {
	s.BaselineTree = newTree
	s.Score = 0
	s.StopTriggered = false
	if newBranch != "" {
		s.BaselineBranch = newBranch
	}
}

// SetViewMode sets the visualization mode.
func (s *SessionState) SetViewMode(mode string) {
	s.ViewMode = mode
}

// GetViewMode returns the current view mode, or empty string if not set.
func (s *SessionState) GetViewMode() string {
	return s.ViewMode
}

// SetViewOpts sets additional view options (flags like "--width 100").
func (s *SessionState) SetViewOpts(opts string) {
	s.ViewOpts = opts
}

// GetViewOpts returns current view options, or empty string if not set.
func (s *SessionState) GetViewOpts() string {
	return s.ViewOpts
}

package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. Restores os.Stdout even if fn panics or calls
// t.Fatal, and drains the pipe concurrently so writes larger than the pipe
// buffer can't deadlock fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		buf.ReadFrom(r)
		close(done)
	}()

	fn()

	w.Close()
	<-done
	return buf.String()
}

// decodeBlockResponse parses a single UserPromptResponse JSON line.
func decodeBlockResponse(t *testing.T, output string) UserPromptResponse {
	t.Helper()
	var resp UserPromptResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", output, err)
	}
	return resp
}

// chdirTemp creates a temp git repo, chdirs into it, and returns a cleanup
// func that restores the original directory. Callers should defer it.
func chdirTemp(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	return tmpDir
}

// headTree returns the current HEAD^{tree} SHA of the repo in cwd.
func headTree(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD^{tree}: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestGetBumperLanesBinPath verifies path detection works.
func TestGetBumperLanesBinPath(t *testing.T) {
	path := getBumperLanesBinPath()

	// Should return non-empty string
	if path == "" {
		t.Error("getBumperLanesBinPath() returned empty string")
	}

	// Should be an absolute path or "bumper-lanes" fallback
	if path != "bumper-lanes" && !filepath.IsAbs(path) {
		t.Errorf("getBumperLanesBinPath() = %q, want absolute path or 'bumper-lanes'", path)
	}
}

// TestHasStatusLineConfigured tests status line detection.
// Uses temp HOME to avoid affecting real user settings.
func TestHasStatusLineConfigured(t *testing.T) {
	// Save and restore HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	t.Run(
		"returns false when no settings file", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when no settings")
			}
		},
	)

	t.Run(
		"returns false when statusLine not configured", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"theme": "dark"}`), 0644,
			)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when statusLine absent")
			}
		},
	)

	t.Run(
		"returns false when statusLine has no command", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"statusLine": {"type": "command"}}`), 0644,
			)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when command missing")
			}
		},
	)

	t.Run(
		"returns false when command is empty string", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"statusLine": {"command": ""}}`), 0644,
			)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when command empty")
			}
		},
	)

	t.Run(
		"returns true when command is configured", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"statusLine": {"command": "/path/to/script.sh"}}`), 0644,
			)

			if !hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = false, want true when command set")
			}
		},
	)
}

// TestHandlePromptNonGitRepo verifies graceful handling in non-git directories.
func TestHandlePromptNonGitRepo(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create and change to a non-git temp directory
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Capture stdout to verify no blocking output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	tests := []struct {
		name   string
		prompt string
	}{
		{"bumper-reset", "/bumper-reset"},
		{"bumper-pause", "/bumper-pause"},
		{"bumper-config", "/bumper-config"},
		{"bumper-tree", "/bumper-tree"},
		{"long form", "/claude-bumper-lanes:bumper-depth"},
		{"non-bumper prompt", "hello world"},
	}

	for _, tc := range tests {
		t.Run(
			tc.name, func(t *testing.T) {
				input := &HookInput{
					SessionID:  "test-session-123",
					UserPrompt: tc.prompt,
				}

				exitCode := HandlePrompt(input)

				if exitCode != 0 {
					t.Errorf("HandlePrompt(%q) = %d, want 0 (pass through)", tc.prompt, exitCode)
				}
			},
		)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "" {
		t.Errorf("HandlePrompt in non-git repo produced output: %q, want empty (pass through)", output)
	}
}

// TestMatchCommand verifies short-form, long-form, and non-matching prompts.
func TestMatchCommand(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		cmdName string
		want    bool
	}{
		{"short form matches", "/bumper-reset", "bumper-reset", true},
		{"long form matches", "/claude-bumper-lanes:bumper-reset", "bumper-reset", true},
		{"short form with trailing args does not match", "/bumper-config 300", "bumper-config", false},
		{"long form with trailing args does not match", "/claude-bumper-lanes:bumper-config 300", "bumper-config", false},
		{"unrelated prompt does not match", "hello world", "bumper-reset", false},
		{"different command name does not match", "/bumper-pause", "bumper-reset", false},
		{"case sensitive - uppercase does not match", "/Bumper-Reset", "bumper-reset", false},
		{"empty prompt does not match", "", "bumper-reset", false},
		{"partial prefix does not match", "/bumper-resetter", "bumper-reset", false},
		{"long form partial prefix does not match", "/claude-bumper-lanes:bumper-resetter", "bumper-reset", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchCommand(tc.prompt, tc.cmdName)
			if got != tc.want {
				t.Errorf("matchCommand(%q, %q) = %v, want %v", tc.prompt, tc.cmdName, got, tc.want)
			}
		})
	}
}

// TestSetThreshold verifies threshold validation, saving, and boundary values.
func TestSetThreshold(t *testing.T) {
	tests := []struct {
		name          string
		valStr        string
		wantContains  string
		wantThreshold int // only checked when wantSaved is true
		wantSaved     bool
	}{
		{"zero disables", "0", "Threshold disabled", 0, true},
		{"boundary 50 is valid (lower bound)", "50", "Threshold set to 50", 50, true},
		{"boundary 2000 is valid (upper bound)", "2000", "Threshold set to 2000", 2000, true},
		{"typical value in range", "600", "Threshold set to 600", 600, true},
		{"boundary 49 is invalid", "49", "must be 0 (disabled) or 50-2000", 0, false},
		{"boundary 2001 is invalid", "2001", "must be 0 (disabled) or 50-2000", 0, false},
		{"negative number is invalid", "-100", "must be 0 (disabled) or 50-2000", 0, false},
		{"non-numeric input is invalid", "abc", "Invalid threshold", 0, false},
		{"whitespace-only input is invalid", "   ", "Invalid threshold", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chdirTemp(t)
			baselineTree := headTree(t)

			sessionID := "test-set-threshold-" + tc.name
			sess, err := state.New(sessionID, baselineTree, "main", 600)
			if err != nil {
				t.Fatalf("state.New() error = %v", err)
			}
			if err := sess.Save(); err != nil {
				t.Fatalf("sess.Save() error = %v", err)
			}

			var exitCode int
			output := captureStdout(t, func() {
				exitCode = setThreshold(sessionID, tc.valStr)
			})

			if exitCode != 0 {
				t.Errorf("setThreshold() = %d, want 0", exitCode)
			}

			resp := decodeBlockResponse(t, output)
			if resp.Decision != "block" {
				t.Errorf("Decision = %q, want %q", resp.Decision, "block")
			}
			if !strings.Contains(resp.Reason, tc.wantContains) {
				t.Errorf("Reason = %q, want substring %q", resp.Reason, tc.wantContains)
			}

			reloaded, err := state.Load(sessionID)
			if err != nil {
				t.Fatalf("state.Load() error = %v", err)
			}
			if tc.wantSaved {
				if reloaded.ThresholdLimit != tc.wantThreshold {
					t.Errorf("session ThresholdLimit = %d, want %d", reloaded.ThresholdLimit, tc.wantThreshold)
				}
				// Durable half: setThreshold's success path calls
				// config.SaveRepoConfig, so the repo's .bumper-lanes.json
				// must reflect the same value, not just in-memory state.
				if got := config.LoadThreshold(); got != tc.wantThreshold {
					t.Errorf("config.LoadThreshold() = %d, want %d (SaveRepoConfig didn't persist)", got, tc.wantThreshold)
				}
			} else {
				if reloaded.ThresholdLimit != 600 {
					t.Errorf("session ThresholdLimit changed to %d on invalid input, want unchanged 600", reloaded.ThresholdLimit)
				}
				if _, err := os.Stat(config.GetConfigPath()); !os.IsNotExist(err) {
					t.Errorf("invalid input must not write %s, but it exists (err=%v)", config.GetConfigPath(), err)
				}
			}
		})
	}
}

// TestHandlePause verifies enforcement pause: state flag, event log, message.
func TestHandlePause(t *testing.T) {
	chdirTemp(t)
	baselineTree := headTree(t)

	sessionID := "test-handle-pause"
	sess, err := state.New(sessionID, baselineTree, "main", 600)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	if err := sess.Save(); err != nil {
		t.Fatalf("sess.Save() error = %v", err)
	}

	var exitCode int
	output := captureStdout(t, func() {
		exitCode = handlePause(sessionID)
	})

	if exitCode != 0 {
		t.Errorf("handlePause() = %d, want 0", exitCode)
	}

	resp := decodeBlockResponse(t, output)
	if resp.Decision != "block" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "block")
	}
	if !strings.Contains(resp.Reason, "Enforcement paused") {
		t.Errorf("Reason = %q, want substring %q", resp.Reason, "Enforcement paused")
	}

	reloaded, err := state.Load(sessionID)
	if err != nil {
		t.Fatalf("state.Load() error = %v", err)
	}
	if !reloaded.Paused {
		t.Error("session Paused = false, want true after handlePause")
	}
}

// TestHandleResume verifies enforcement resume: state flag, message.
func TestHandleResume(t *testing.T) {
	chdirTemp(t)
	baselineTree := headTree(t)

	sessionID := "test-handle-resume"
	sess, err := state.New(sessionID, baselineTree, "main", 600)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	sess.SetPaused(true)
	sess.Score = 42
	if err := sess.Save(); err != nil {
		t.Fatalf("sess.Save() error = %v", err)
	}

	var exitCode int
	output := captureStdout(t, func() {
		exitCode = handleResume(sessionID)
	})

	if exitCode != 0 {
		t.Errorf("handleResume() = %d, want 0", exitCode)
	}

	resp := decodeBlockResponse(t, output)
	if resp.Decision != "block" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "block")
	}
	if !strings.Contains(resp.Reason, "Enforcement resumed") {
		t.Errorf("Reason = %q, want substring %q", resp.Reason, "Enforcement resumed")
	}
	if !strings.Contains(resp.Reason, "42/600") {
		t.Errorf("Reason = %q, want score/limit substring %q", resp.Reason, "42/600")
	}

	reloaded, err := state.Load(sessionID)
	if err != nil {
		t.Fatalf("state.Load() error = %v", err)
	}
	if reloaded.Paused {
		t.Error("session Paused = true, want false after handleResume")
	}
}

// TestHandleConfigShow verifies the no-args config display path.
func TestHandleConfigShow(t *testing.T) {
	// Isolate from any real global config or plugin env so the "default"
	// source assertion holds regardless of the host machine's state.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, key := range []string{"CLAUDE_PLUGIN_OPTION_THRESHOLD", "CLAUDE_PLUGIN_OPTION_RESET_ON", "CLAUDE_PLUGIN_OPTION_ON_TRIP", "CLAUDE_PLUGIN_OPTION_MAX_AUTO_REVIEWS", "CLAUDE_PLUGIN_OPTION_REVIEW_COMMAND", "CLAUDE_PLUGIN_OPTION_TRIPWIRES_BLOCK_AUTO_REVIEW", "CLAUDE_PLUGIN_OPTION_STATUSLINE_AUTO_SETUP"} {
		t.Setenv(key, "")
	}

	chdirTemp(t)
	baselineTree := headTree(t)

	sessionID := "test-handle-config-show"
	sess, err := state.New(sessionID, baselineTree, "main", 600)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	if err := sess.Save(); err != nil {
		t.Fatalf("sess.Save() error = %v", err)
	}

	var exitCode int
	output := captureStdout(t, func() {
		exitCode = handleConfig(sessionID, "")
	})

	if exitCode != 0 {
		t.Errorf("handleConfig() = %d, want 0", exitCode)
	}

	resp := decodeBlockResponse(t, output)
	for _, want := range []string{"Threshold:", "Reset policy:", "Source:"} {
		if !strings.Contains(resp.Reason, want) {
			t.Errorf("Reason = %q, want substring %q", resp.Reason, want)
		}
	}
	if !strings.Contains(resp.Reason, "default") {
		t.Errorf("Reason = %q, want source %q (no config file present)", resp.Reason, "default")
	}
}

// TestHandleConfigWithArg verifies handleConfig delegates numeric args to setThreshold.
func TestHandleConfigWithArg(t *testing.T) {
	chdirTemp(t)
	baselineTree := headTree(t)

	sessionID := "test-handle-config-arg"
	sess, err := state.New(sessionID, baselineTree, "main", 600)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	if err := sess.Save(); err != nil {
		t.Fatalf("sess.Save() error = %v", err)
	}

	var exitCode int
	output := captureStdout(t, func() {
		exitCode = handleConfig(sessionID, "300")
	})

	if exitCode != 0 {
		t.Errorf("handleConfig() = %d, want 0", exitCode)
	}

	resp := decodeBlockResponse(t, output)
	if !strings.Contains(resp.Reason, "Threshold set to 300") {
		t.Errorf("Reason = %q, want substring %q", resp.Reason, "Threshold set to 300")
	}

	reloaded, err := state.Load(sessionID)
	if err != nil {
		t.Fatalf("state.Load() error = %v", err)
	}
	if reloaded.ThresholdLimit != 300 {
		t.Errorf("session ThresholdLimit = %d, want 300", reloaded.ThresholdLimit)
	}
}

// TestHandleDiff verifies the no-changes and has-changes paths.
func TestHandleDiff(t *testing.T) {
	t.Run("no changes against baseline", func(t *testing.T) {
		chdirTemp(t)
		baselineTree := headTree(t)

		sessionID := "test-handle-diff-clean"
		sess, err := state.New(sessionID, baselineTree, "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = handleDiff(sessionID)
		})

		if exitCode != 0 {
			t.Errorf("handleDiff() = %d, want 0", exitCode)
		}

		resp := decodeBlockResponse(t, output)
		if resp.Reason != "No changes against the review baseline." {
			t.Errorf("Reason = %q, want %q", resp.Reason, "No changes against the review baseline.")
		}
	})

	t.Run("renders diff for uncommitted changes", func(t *testing.T) {
		tmpDir := chdirTemp(t)
		baselineTree := headTree(t)

		if err := os.WriteFile(filepath.Join(tmpDir, "new-file.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		sessionID := "test-handle-diff-dirty"
		sess, err := state.New(sessionID, baselineTree, "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = handleDiff(sessionID)
		})

		if exitCode != 0 {
			t.Errorf("handleDiff() = %d, want 0", exitCode)
		}

		resp := decodeBlockResponse(t, output)
		if resp.Reason == "" || resp.Reason == "No changes against the review baseline." {
			t.Errorf("Reason = %q, want non-empty rendered diff", resp.Reason)
		}
		if !strings.Contains(resp.Reason, "new-file.go") {
			t.Errorf("Reason = %q, want it to mention the changed file", resp.Reason)
		}
	})
}

// TestHandlePromptDispatch covers HandlePrompt's routing table end to end.
func TestHandlePromptDispatch(t *testing.T) {
	t.Run("unknown bumper command passes through with no output", func(t *testing.T) {
		chdirTemp(t)
		baselineTree := headTree(t)

		sessionID := "test-dispatch-unknown"
		sess, err := state.New(sessionID, baselineTree, "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = HandlePrompt(&HookInput{SessionID: sessionID, UserPrompt: "/bumper-foo"})
		})

		if exitCode != 0 {
			t.Errorf("HandlePrompt() = %d, want 0", exitCode)
		}
		if output != "" {
			t.Errorf("HandlePrompt(/bumper-foo) output = %q, want empty", output)
		}
	})

	t.Run("non-command prompt injects budget context above 50%% usage", func(t *testing.T) {
		chdirTemp(t)
		baselineTree := headTree(t)

		sessionID := "test-dispatch-budget"
		sess, err := state.New(sessionID, baselineTree, "main", 100)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.Score = 60 // 60% used, over the 50% budget-context threshold
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = HandlePrompt(&HookInput{SessionID: sessionID, UserPrompt: "implement the next feature"})
		})

		if exitCode != 0 {
			t.Errorf("HandlePrompt() = %d, want 0", exitCode)
		}

		var resp ContextResponse
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("failed to decode context response %q: %v", output, err)
		}
		if resp.HookSpecificOutput.HookEventName != "UserPromptSubmit" {
			t.Errorf("HookEventName = %q, want %q", resp.HookSpecificOutput.HookEventName, "UserPromptSubmit")
		}
		if !strings.Contains(resp.HookSpecificOutput.AdditionalContext, "review-budget pts remain") {
			t.Errorf("AdditionalContext = %q, want budget line", resp.HookSpecificOutput.AdditionalContext)
		}
	})

	t.Run("non-command prompt below 50%% usage produces no output", func(t *testing.T) {
		chdirTemp(t)
		baselineTree := headTree(t)

		sessionID := "test-dispatch-no-budget"
		sess, err := state.New(sessionID, baselineTree, "main", 100)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.Score = 10 // well under 50%
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = HandlePrompt(&HookInput{SessionID: sessionID, UserPrompt: "implement the next feature"})
		})

		if exitCode != 0 {
			t.Errorf("HandlePrompt() = %d, want 0", exitCode)
		}
		if output != "" {
			t.Errorf("HandlePrompt() output = %q, want empty when under budget threshold", output)
		}
	})

	t.Run("bumper-reset command dispatches to handleReset", func(t *testing.T) {
		chdirTemp(t)
		baselineTree := headTree(t)

		sessionID := "test-dispatch-reset"
		sess, err := state.New(sessionID, baselineTree, "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.Score = 123
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = HandlePrompt(&HookInput{SessionID: sessionID, UserPrompt: "/bumper-reset"})
		})

		if exitCode != 0 {
			t.Errorf("HandlePrompt() = %d, want 0", exitCode)
		}
		resp := decodeBlockResponse(t, output)
		if !strings.Contains(resp.Reason, "Baseline reset") {
			t.Errorf("Reason = %q, want substring %q", resp.Reason, "Baseline reset")
		}

		reloaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("state.Load() error = %v", err)
		}
		if reloaded.Score != 0 {
			t.Errorf("session Score = %d, want 0 after reset", reloaded.Score)
		}
	})

	t.Run("bumper-diff command dispatches to handleDiff", func(t *testing.T) {
		chdirTemp(t)
		baselineTree := headTree(t)

		sessionID := "test-dispatch-diff"
		sess, err := state.New(sessionID, baselineTree, "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = HandlePrompt(&HookInput{SessionID: sessionID, UserPrompt: "/claude-bumper-lanes:bumper-diff"})
		})

		if exitCode != 0 {
			t.Errorf("HandlePrompt() = %d, want 0", exitCode)
		}
		resp := decodeBlockResponse(t, output)
		if resp.Reason != "No changes against the review baseline." {
			t.Errorf("Reason = %q, want %q", resp.Reason, "No changes against the review baseline.")
		}
	})

	t.Run("bumper-config command with arg dispatches to setThreshold", func(t *testing.T) {
		chdirTemp(t)
		baselineTree := headTree(t)

		sessionID := "test-dispatch-config"
		sess, err := state.New(sessionID, baselineTree, "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("sess.Save() error = %v", err)
		}

		var exitCode int
		output := captureStdout(t, func() {
			exitCode = HandlePrompt(&HookInput{SessionID: sessionID, UserPrompt: "/bumper-config 250"})
		})

		if exitCode != 0 {
			t.Errorf("HandlePrompt() = %d, want 0", exitCode)
		}
		resp := decodeBlockResponse(t, output)
		if !strings.Contains(resp.Reason, "Threshold set to 250") {
			t.Errorf("Reason = %q, want substring %q", resp.Reason, "Threshold set to 250")
		}
	})
}

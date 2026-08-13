package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

func TestFuelGaugeTier(t *testing.T) {
	threshold := 400

	// Tiers: <70% silent, 70-89% NOTICE, 90%+ WARNING
	tests := []struct {
		name      string
		score     int
		wantTier  string
		wantQuiet bool
	}{
		{
			name:      "0% - silent",
			score:     0,
			wantTier:  "",
			wantQuiet: true,
		},
		{
			name:      "25% - silent",
			score:     100,
			wantTier:  "",
			wantQuiet: true,
		},
		{
			name:      "69% - silent",
			score:     276,
			wantTier:  "",
			wantQuiet: true,
		},
		{
			name:      "70% - notice",
			score:     280,
			wantTier:  "NOTICE",
			wantQuiet: false,
		},
		{
			name:      "80% - notice",
			score:     320,
			wantTier:  "NOTICE",
			wantQuiet: false,
		},
		{
			name:      "89% - notice",
			score:     356,
			wantTier:  "NOTICE",
			wantQuiet: false,
		},
		{
			name:      "90% - warning",
			score:     360,
			wantTier:  "WARNING",
			wantQuiet: false,
		},
		{
			name:      "100% - warning",
			score:     400,
			wantTier:  "WARNING",
			wantQuiet: false,
		},
		{
			name:      "150% - warning",
			score:     600,
			wantTier:  "WARNING",
			wantQuiet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, quiet := getFuelGaugeTier(tt.score, threshold)
			if tier != tt.wantTier {
				t.Errorf("getFuelGaugeTier(%d, %d) tier = %q, want %q", tt.score, threshold, tier, tt.wantTier)
			}
			if quiet != tt.wantQuiet {
				t.Errorf("getFuelGaugeTier(%d, %d) quiet = %v, want %v", tt.score, threshold, quiet, tt.wantQuiet)
			}
		})
	}
}

func TestFuelGaugeMessage(t *testing.T) {
	tests := []struct {
		tier        string
		score       int
		threshold   int
		wantContain string
	}{
		{"NOTICE", 220, 400, "55%"},
		{"WARNING", 320, 400, "80%"},
		{"CRITICAL", 380, 400, "95%"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			msg := formatFuelGaugeMessage(tt.tier, tt.score, tt.threshold)
			if !strings.Contains(msg, tt.tier) {
				t.Errorf("message should contain tier %q, got: %s", tt.tier, msg)
			}
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("message should contain %q, got: %s", tt.wantContain, msg)
			}
		})
	}
}

// getFuelGaugeTier calculates the warning tier based on score vs threshold
// Tiers: 70% NOTICE, 90% WARNING
func getFuelGaugeTier(score, threshold int) (tier string, quiet bool) {
	if threshold <= 0 {
		return "", true
	}

	percent := (score * 100) / threshold

	switch {
	case percent >= 90:
		return "WARNING", false
	case percent >= 70:
		return "NOTICE", false
	default:
		return "", true
	}
}

// formatFuelGaugeMessage creates the warning message
func formatFuelGaugeMessage(tier string, score, threshold int) string {
	percent := (score * 100) / threshold
	return tier + ": Review budget at " + itoa(percent) + "%. " + itoa(score) + "/" + itoa(threshold) + " pts."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func TestGitCommitPattern(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Should match
		{"simple git commit", "git commit -m 'test'", true},
		{"git commit with message", `git commit -m "feat: add feature"`, true},
		{"git commit all", "git commit -a -m 'changes'", true},
		{"git commit amend", "git commit --amend", true},
		{"git -C path commit", "git -C /some/path commit -m 'msg'", true},
		{"git with git-dir", "git --git-dir=/x commit -m 'y'", true},
		{"commit with multiple flags", "git -C /path --work-tree=/other commit -m 'z'", true},

		// Should NOT match
		{"git status", "git status", false},
		{"git diff", "git diff HEAD", false},
		{"prose about git commit", "use git to commit your changes", false},
		{"commitizen command", "cz commit", false},
		{"random commit word", "I will commit to this", false},
		{"git log with commit", "git log --oneline | grep commit", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gitCommitPattern.MatchString(tt.command)
			if got != tt.want {
				t.Errorf("gitCommitPattern.MatchString(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestNoVerifyDetection(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"long flag", "git commit --no-verify -m 'x'", true},
		{"short flag", "git commit -n -m 'x'", true},
		{"bundled short flag", "git commit -an -m 'x'", true},
		{"plain commit", "git commit -m 'x'", false},
		{"quiet commit", "git commit -q -m 'x'", false},
		{"amend", "git commit --amend", false},
		{"no-edit", "git commit --amend --no-edit", false},
		{"-n in later pipeline command", "git commit -m 'x' && git log -n 1", false},
		{"-n after pipe", "git commit -m 'x' | head -n 20", false},
		{"-n after semicolon", "git commit -m 'x'; echo -n done", false},
		{"no-verify in commit segment of compound", "git add -u && git commit --no-verify -m 'x' && git push", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noVerifyPattern.MatchString(commitSegment(tt.command)); got != tt.want {
				t.Errorf("noVerify(commitSegment(%q)) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestPostToolUseRouting(t *testing.T) {
	t.Run("Write routes to file handler", func(t *testing.T) {
		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Write",
			SessionID:     "nonexistent-session-123",
		}

		// Should not panic, just return 0 (fail open - no session)
		exitCode := PostToolUse(input)
		if exitCode != 0 {
			t.Errorf("PostToolUse(Write) = %d, want 0 (fail open)", exitCode)
		}
	})

	t.Run("Edit routes to file handler", func(t *testing.T) {
		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Edit",
			SessionID:     "nonexistent-session-456",
		}

		exitCode := PostToolUse(input)
		if exitCode != 0 {
			t.Errorf("PostToolUse(Edit) = %d, want 0 (fail open)", exitCode)
		}
	})

	t.Run("Bash routes to commit handler", func(t *testing.T) {
		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     "nonexistent-session-789",
			ToolInput:     &hookio.ToolInput{Command: "git status"}, // not a commit
		}

		exitCode := PostToolUse(input)
		if exitCode != 0 {
			t.Errorf("PostToolUse(Bash non-commit) = %d, want 0", exitCode)
		}
	})

	t.Run("Other tools return 0", func(t *testing.T) {
		for _, tool := range []string{"Read", "Glob", "Grep", "List", "Search"} {
			input := &hookio.Input{
				HookEventName: "PostToolUse",
				ToolName:      tool,
				SessionID:     "any-session",
			}

			exitCode := PostToolUse(input)
			if exitCode != 0 {
				t.Errorf("PostToolUse(%s) = %d, want 0", tool, exitCode)
			}
		}
	})

	t.Run("Wrong hook event returns 0", func(t *testing.T) {
		input := &hookio.Input{
			HookEventName: "Stop",
			ToolName:      "Write",
			SessionID:     "any-session",
		}

		exitCode := PostToolUse(input)
		if exitCode != 0 {
			t.Errorf("PostToolUse(wrong event) = %d, want 0", exitCode)
		}
	})
}

// TestNotebookEditUpdatesScore verifies NotebookEdit routes to the fuel gauge
// (the hooks.json matcher includes it; the handler must too).
func TestNotebookEditUpdatesScore(t *testing.T) {
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	os.WriteFile("initial.txt", []byte("initial\n"), 0644)
	exec.Command("git", "add", "initial.txt").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	sessionID := "test-notebookedit-score"
	baseline, _ := git.CaptureTree()
	sess, err := state.New(sessionID, baseline, "main", 400)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sess.Save()

	// Dirty the tree so a fresh score calculation is non-zero
	var content strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&content, "// line %d\n", i)
	}
	os.WriteFile("notebook-cells.txt", []byte(content.String()), 0644)

	input := &hookio.Input{
		HookEventName: "PostToolUse",
		ToolName:      "NotebookEdit",
		SessionID:     sessionID,
	}
	if exitCode := PostToolUse(input); exitCode != 0 {
		t.Errorf("PostToolUse(NotebookEdit) = %d, want 0", exitCode)
	}

	reloaded, _ := state.Load(sessionID)
	if reloaded.Score == 0 {
		t.Errorf("Score = 0 after NotebookEdit with dirty tree, want > 0 (fuel gauge ran)")
	}
}

// gitCommit runs git commit in the current directory and fails the test on error.
func gitCommit(t *testing.T, args ...string) {
	t.Helper()
	if err := exec.Command("git", append([]string{"commit"}, args...)...).Run(); err != nil {
		t.Fatalf("git commit %v failed: %v", args, err)
	}
}

func TestHandleBashCommit(t *testing.T) {
	t.Run("auto-resets baseline when HEAD moved", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		sessionID := "test-bash-commit"
		sess, err := state.New(sessionID, "old-tree-sha", "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Score = 100
		sess.HeadBeforeCommit = git.HeadCommit() // what PreToolUse records
		if err := sess.Save(); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Actually commit so HEAD moves (quiet: evidence is HEAD, not output)
		os.WriteFile("committed.txt", []byte("x\n"), 0644)
		exec.Command("git", "add", "committed.txt").Run()
		gitCommit(t, "-q", "-m", "landed")

		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git commit -q -m 'landed'"},
		}

		if exitCode := PostToolUse(input); exitCode != 0 {
			t.Errorf("PostToolUse(git commit) = %d, want 0", exitCode)
		}

		reloaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("Failed to reload session: %v", err)
		}

		output, _ := exec.Command("git", "rev-parse", "HEAD^{tree}").Output()
		expectedTree := strings.TrimSpace(string(output))
		if reloaded.BaselineTree != expectedTree {
			t.Errorf("BaselineTree = %q, want %q (HEAD^{tree})", reloaded.BaselineTree, expectedTree)
		}
		if reloaded.Score != 0 {
			t.Errorf("Score = %d, want 0 (reset)", reloaded.Score)
		}
		if reloaded.HeadBeforeCommit != "" {
			t.Errorf("HeadBeforeCommit = %q, want cleared", reloaded.HeadBeforeCommit)
		}

		// Reset must be recorded in the event log with score-at-reset
		data, err := os.ReadFile(filepath.Join(tmpDir, ".git", "bumper-checkpoints", "events.jsonl"))
		if err != nil {
			t.Fatalf("events.jsonl not written: %v", err)
		}
		var entry struct {
			Event string `json:"event"`
			Score int    `json:"score"`
			Cause string `json:"cause"`
		}
		lastLine := strings.TrimSpace(string(data))
		if i := strings.LastIndex(lastLine, "\n"); i >= 0 {
			lastLine = lastLine[i+1:]
		}
		if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
			t.Fatalf("events.jsonl line not valid JSON: %v", err)
		}
		if entry.Event != "reset" || entry.Cause != "commit" || entry.Score != 100 {
			t.Errorf("event = %+v, want reset/commit with score 100", entry)
		}
	})

	t.Run("rejected commit (HEAD unmoved) does not reset baseline", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		sessionID := "test-bash-failed-commit"
		sess, err := state.New(sessionID, "old-tree-sha", "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Score = 100
		sess.HeadBeforeCommit = git.HeadCommit()
		sess.Save()

		// No commit performed: HEAD stays put (as after a pre-commit rejection)
		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git diff --stat && git commit -m 'rejected'"},
		}

		if exitCode := PostToolUse(input); exitCode != 0 {
			t.Errorf("PostToolUse(failed commit) = %d, want 0", exitCode)
		}

		reloaded, _ := state.Load(sessionID)
		if reloaded.BaselineTree != "old-tree-sha" {
			t.Errorf("BaselineTree = %q, want unchanged old-tree-sha", reloaded.BaselineTree)
		}
		if reloaded.Score != 100 {
			t.Errorf("Score = %d, want 100 (unchanged)", reloaded.Score)
		}
		if reloaded.HeadBeforeCommit != "" {
			t.Errorf("HeadBeforeCommit = %q, want cleared after the attempt", reloaded.HeadBeforeCommit)
		}
	})

	t.Run("commit without recorded pre-HEAD does not reset baseline", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		sessionID := "test-bash-no-prehead"
		sess, _ := state.New(sessionID, "old-tree-sha", "main", 400)
		sess.Score = 100
		sess.Save() // HeadBeforeCommit never recorded

		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git commit -m 'unproven'"},
		}

		PostToolUse(input)
		reloaded, _ := state.Load(sessionID)
		if reloaded.BaselineTree != "old-tree-sha" {
			t.Errorf("BaselineTree = %q, want unchanged (no evidence)", reloaded.BaselineTree)
		}
	})

	t.Run("reset_on=human never resets on commit", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)
		os.WriteFile(".bumper-lanes.json", []byte(`{"reset_on": "human"}`), 0644)

		sessionID := "test-bash-human-policy"
		sess, _ := state.New(sessionID, "old-tree-sha", "main", 400)
		sess.Score = 100
		sess.HeadBeforeCommit = git.HeadCommit()
		sess.Save()

		gitCommit(t, "--allow-empty", "-m", "ok") // HEAD moves: real evidence

		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git commit -m 'ok'"},
		}

		PostToolUse(input)
		reloaded, _ := state.Load(sessionID)
		if reloaded.BaselineTree != "old-tree-sha" {
			t.Errorf("BaselineTree = %q, want unchanged under reset_on=human", reloaded.BaselineTree)
		}
		if reloaded.Score != 100 {
			t.Errorf("Score = %d, want 100 (unchanged)", reloaded.Score)
		}
	})

	t.Run("reset_on=verified-commit refuses --no-verify", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)
		os.WriteFile(".bumper-lanes.json", []byte(`{"reset_on": "verified-commit"}`), 0644)

		sessionID := "test-bash-noverify"
		sess, _ := state.New(sessionID, "old-tree-sha", "main", 400)
		sess.Score = 100
		sess.HeadBeforeCommit = git.HeadCommit()
		sess.Save()

		gitCommit(t, "--allow-empty", "--no-verify", "-m", "sneaky")

		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git commit --no-verify -m 'sneaky'"},
		}

		PostToolUse(input)
		reloaded, _ := state.Load(sessionID)
		if reloaded.BaselineTree != "old-tree-sha" {
			t.Errorf("BaselineTree = %q, want unchanged when hooks bypassed", reloaded.BaselineTree)
		}
	})

	t.Run("reset_on=verified-commit resets on hook-verified commit", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)
		os.WriteFile(".bumper-lanes.json", []byte(`{"reset_on": "verified-commit"}`), 0644)

		sessionID := "test-bash-verified-ok"
		sess, _ := state.New(sessionID, "old-tree-sha", "main", 400)
		sess.Score = 100
		sess.HeadBeforeCommit = git.HeadCommit()
		sess.Save()

		gitCommit(t, "--allow-empty", "-m", "clean")

		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git commit -m 'clean'"},
		}

		PostToolUse(input)
		reloaded, _ := state.Load(sessionID)
		if reloaded.BaselineTree == "old-tree-sha" {
			t.Errorf("BaselineTree unchanged, want reset for verified commit")
		}
	})

	t.Run("PreToolUse records HEAD for commit-shaped Bash commands", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		sessionID := "test-pre-records-head"
		sess, _ := state.New(sessionID, "old-tree-sha", "main", 400)
		sess.Save()

		input := &hookio.Input{
			HookEventName: "PreToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git commit -m 'about to run'"},
		}
		if exitCode := PreToolUse(input); exitCode != 0 {
			t.Errorf("PreToolUse(Bash commit) = %d, want 0 (never blocks Bash)", exitCode)
		}

		reloaded, _ := state.Load(sessionID)
		if reloaded.HeadBeforeCommit != git.HeadCommit() {
			t.Errorf("HeadBeforeCommit = %q, want current HEAD %q", reloaded.HeadBeforeCommit, git.HeadCommit())
		}

		// Non-commit Bash must not record
		sess2, _ := state.New("test-pre-no-record", "old-tree-sha", "main", 400)
		sess2.Save()
		PreToolUse(&hookio.Input{
			HookEventName: "PreToolUse",
			ToolName:      "Bash",
			SessionID:     "test-pre-no-record",
			ToolInput:     &hookio.ToolInput{Command: "git status"},
		})
		reloaded2, _ := state.Load("test-pre-no-record")
		if reloaded2.HeadBeforeCommit != "" {
			t.Errorf("HeadBeforeCommit = %q for non-commit command, want empty", reloaded2.HeadBeforeCommit)
		}
	})

	t.Run("non-commit bash commands ignored", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		sessionID := "test-bash-nocommit"
		sess, err := state.New(sessionID, "original-tree", "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Score = 50
		sess.Save()

		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &hookio.ToolInput{Command: "git status"},
		}

		exitCode := PostToolUse(input)
		if exitCode != 0 {
			t.Errorf("PostToolUse(git status) = %d, want 0", exitCode)
		}

		// Session should be unchanged
		reloaded, _ := state.Load(sessionID)
		if reloaded.BaselineTree != "original-tree" {
			t.Errorf("BaselineTree changed unexpectedly to %q", reloaded.BaselineTree)
		}
		if reloaded.Score != 50 {
			t.Errorf("Score = %d, want 50 (unchanged)", reloaded.Score)
		}
	})

	t.Run("missing command fails open", func(t *testing.T) {
		input := &hookio.Input{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     "any",
			ToolInput:     nil, // no tool input
		}

		exitCode := PostToolUse(input)
		if exitCode != 0 {
			t.Errorf("PostToolUse(nil input) = %d, want 0 (fail open)", exitCode)
		}
	})
}

// TestTripwireDetection verifies the manual-check requirement from the
// audit: a t.Skip insertion at a tiny score must surface immediately.
func TestTripwireDetection(t *testing.T) {
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Tracked test file in the baseline
	os.WriteFile("thing_test.go", []byte("package thing\n\nfunc TestA(t *testing.T) {\n}\n"), 0644)
	exec.Command("git", "add", "thing_test.go").Run()
	gitCommit(t, "-m", "baseline")

	sessionID := "test-tripwires"
	baseline, _ := git.CaptureTree()
	sess, err := state.New(sessionID, baseline, "main", 600)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sess.Save()

	// Two-line high-risk edit: add a test skip to a tracked file...
	os.WriteFile("thing_test.go", []byte("package thing\n\nfunc TestA(t *testing.T) {\n\tt.Skip(\"flaky\")\n}\n"), 0644)
	// ...and touch a CI workflow (untracked - caught by the path lane)
	os.MkdirAll(".github/workflows", 0755)
	os.WriteFile(".github/workflows/ci.yml", []byte("on: push\n"), 0644)

	input := &hookio.Input{
		HookEventName: "PostToolUse",
		ToolName:      "Edit",
		SessionID:     sessionID,
	}
	if exitCode := PostToolUse(input); exitCode != 0 {
		t.Errorf("PostToolUse(Edit) = %d, want 0", exitCode)
	}

	reloaded, _ := state.Load(sessionID)
	joined := strings.Join(reloaded.Tripwires, "\n")
	if !strings.Contains(joined, ".github/workflows/ci.yml") {
		t.Errorf("Tripwires = %v, want CI workflow path hit", reloaded.Tripwires)
	}
	if !strings.Contains(joined, "t.Skip (thing_test.go)") {
		t.Errorf("Tripwires = %v, want added-line t.Skip hit", reloaded.Tripwires)
	}

	// Second run: same hits are already known, no new ones recorded
	before := len(reloaded.Tripwires)
	PostToolUse(input)
	again, _ := state.Load(sessionID)
	if len(again.Tripwires) != before {
		t.Errorf("Tripwires grew from %d to %d on unchanged diff, want stable", before, len(again.Tripwires))
	}

	// Reset clears them
	again.ResetBaseline("new-tree", "main", "")
	if len(again.Tripwires) != 0 {
		t.Errorf("Tripwires = %v after reset, want empty", again.Tripwires)
	}
}

func TestTripwiresDisabledByEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)
	os.WriteFile(".bumper-lanes.json", []byte(`{"tripwire_paths": [], "tripwire_patterns": []}`), 0644)

	sessionID := "test-tripwires-off"
	baseline, _ := git.CaptureTree()
	sess, _ := state.New(sessionID, baseline, "main", 600)
	sess.Save()

	os.MkdirAll(".github/workflows", 0755)
	os.WriteFile(".github/workflows/ci.yml", []byte("on: push\n"), 0644)

	PostToolUse(&hookio.Input{HookEventName: "PostToolUse", ToolName: "Edit", SessionID: sessionID})

	reloaded, _ := state.Load(sessionID)
	if len(reloaded.Tripwires) != 0 {
		t.Errorf("Tripwires = %v with tripwires disabled, want empty", reloaded.Tripwires)
	}
}

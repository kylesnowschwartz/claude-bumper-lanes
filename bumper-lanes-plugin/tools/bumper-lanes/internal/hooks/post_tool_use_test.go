package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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

func TestPostToolUseRouting(t *testing.T) {
	t.Run("Write routes to file handler", func(t *testing.T) {
		input := &HookInput{
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
		input := &HookInput{
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
		input := &HookInput{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     "nonexistent-session-789",
			ToolInput:     &ToolInput{Command: "git status"}, // not a commit
		}

		exitCode := PostToolUse(input)
		if exitCode != 0 {
			t.Errorf("PostToolUse(Bash non-commit) = %d, want 0", exitCode)
		}
	})

	t.Run("Other tools return 0", func(t *testing.T) {
		for _, tool := range []string{"Read", "Glob", "Grep", "List", "Search"} {
			input := &HookInput{
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
		input := &HookInput{
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

func TestAutoResetOnCleanTree(t *testing.T) {
	// Skip if not in a git repo
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("auto-resets after external commit (RED - should fail initially)", func(t *testing.T) {
		// This test captures the ACTUAL user scenario:
		// 1. Session starts at tree-A
		// 2. Claude makes changes (working tree dirty)
		// 3. User commits externally (HEAD advances to tree-B)
		// 4. Working tree is now clean (at tree-B)
		// 5. Claude's next Write/Edit should auto-reset (working tree == HEAD)

		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Initial commit to establish tree-A
		os.WriteFile("initial.txt", []byte("initial\n"), 0644)
		exec.Command("git", "add", "initial.txt").Run()
		exec.Command("git", "commit", "-m", "initial").Run()

		// Capture baseline at tree-A (session start)
		baseline, err := CaptureTree()
		if err != nil {
			t.Fatalf("Failed to capture baseline: %v", err)
		}

		// Create session with baseline = tree-A
		sessionID := "test-external-commit"
		sess, err := state.New(sessionID, baseline, "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Save()

		// Simulate Claude making changes (dirty working tree)
		os.WriteFile("feature.txt", []byte("new feature\n"), 0644)

		// Verify score is non-zero (changes exist)
		input := &HookInput{
			HookEventName: "PostToolUse",
			ToolName:      "Write",
			SessionID:     sessionID,
		}
		handleWriteEdit(input) // This updates score

		reloaded, _ := state.Load(sessionID)
		if reloaded.Score == 0 {
			t.Fatalf("Score should be non-zero after Claude makes changes")
		}
		originalScore := reloaded.Score
		t.Logf("Score after changes: %d", originalScore)

		// User commits externally (NOT via Claude's Bash tool)
		exec.Command("git", "add", "feature.txt").Run()
		commitCmd := exec.Command("git", "commit", "-m", "feat: add feature")
		if err := commitCmd.Run(); err != nil {
			t.Fatalf("External commit failed: %v", err)
		}

		// Working tree is now CLEAN (no uncommitted changes)
		// But baseline is still tree-A, HEAD is now tree-B
		// Score = diff(tree-A, tree-B) = original score (non-zero)

		// Claude's next Write/Edit should detect clean tree and auto-reset
		exitCode := handleWriteEdit(input)

		// EXPECTATION: Should return 2 with auto-reset message
		if exitCode != 2 {
			t.Errorf("handleWriteEdit(clean tree after external commit) = %d, want 2 (auto-reset)", exitCode)
		}

		// Verify session was reset
		reloaded, err = state.Load(sessionID)
		if err != nil {
			t.Fatalf("Failed to reload session: %v", err)
		}

		// Score should be reset to 0
		if reloaded.Score != 0 {
			t.Errorf("Score = %d, want 0 (should auto-reset after external commit)", reloaded.Score)
		}

		// Baseline should be updated to HEAD (tree-B)
		headTree, _ := exec.Command("git", "rev-parse", "HEAD^{tree}").Output()
		expectedTree := strings.TrimSpace(string(headTree))
		if reloaded.BaselineTree != expectedTree {
			t.Errorf("BaselineTree = %q, want %q (HEAD after commit)", reloaded.BaselineTree, expectedTree)
		}
	})

	t.Run("auto-resets when working tree matches HEAD (git reset --hard)", func(t *testing.T) {
		// Edge case: User does git reset --hard HEAD, bringing working tree
		// back to a clean state matching HEAD (which happens to be the baseline)
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create initial commit
		os.WriteFile("initial.txt", []byte("initial\n"), 0644)
		exec.Command("git", "add", "initial.txt").Run()
		exec.Command("git", "commit", "-m", "initial").Run()

		// Capture baseline = HEAD
		baseline, err := CaptureTree()
		if err != nil {
			t.Fatalf("Failed to capture baseline: %v", err)
		}

		// Create session
		sessionID := "test-reset-hard"
		sess, err := state.New(sessionID, baseline, "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Score = 100 // Pretend we had accumulated score
		sess.Save()

		// Working tree is clean (matches HEAD == baseline)
		// This simulates: user made changes, then did git reset --hard HEAD
		input := &HookInput{
			HookEventName: "PostToolUse",
			ToolName:      "Write",
			SessionID:     sessionID,
		}

		exitCode := handleWriteEdit(input)
		// Should return 2 with auto-reset message
		if exitCode != 2 {
			t.Errorf("handleWriteEdit(clean tree) = %d, want 2 (auto-reset)", exitCode)
		}

		// Verify session was reset
		reloaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("Failed to reload session: %v", err)
		}

		if reloaded.Score != 0 {
			t.Errorf("Score = %d, want 0 (auto-reset)", reloaded.Score)
		}

		// Baseline should still be HEAD
		headTree := GetHeadTree()
		if reloaded.BaselineTree != headTree {
			t.Errorf("BaselineTree = %q, want %q (HEAD)", reloaded.BaselineTree, headTree)
		}
	})

	t.Run("does not reset when working tree is dirty (uncommitted changes)", func(t *testing.T) {
		// Verify we DON'T auto-reset when working tree has uncommitted changes
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create initial commit
		os.WriteFile("initial.txt", []byte("initial\n"), 0644)
		exec.Command("git", "add", "initial.txt").Run()
		exec.Command("git", "commit", "-m", "initial").Run()

		// Capture baseline at HEAD
		baseline, err := CaptureTree()
		if err != nil {
			t.Fatalf("Failed to capture baseline: %v", err)
		}

		// Create session
		sessionID := "test-no-reset-dirty"
		sess, err := state.New(sessionID, baseline, "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Save()

		// Make uncommitted changes (working tree != HEAD)
		os.WriteFile(filepath.Join(tmpDir, "uncommitted.txt"), []byte("new content\n"), 0644)

		input := &HookInput{
			HookEventName: "PostToolUse",
			ToolName:      "Write",
			SessionID:     sessionID,
		}

		exitCode := handleWriteEdit(input)
		// Should return 0 (under 70% threshold) - NOT auto-reset
		if exitCode != 0 {
			t.Errorf("handleWriteEdit(dirty tree) = %d, want 0 (under threshold, no auto-reset)", exitCode)
		}

		// Verify baseline NOT changed
		reloaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("Failed to reload session: %v", err)
		}

		if reloaded.BaselineTree != baseline {
			t.Errorf("BaselineTree changed unexpectedly to %q, want %q (should not reset)", reloaded.BaselineTree, baseline)
		}

		// Score should be updated (non-zero because working tree is dirty)
		if reloaded.Score == 0 {
			t.Errorf("Score = 0, want non-zero (dirty tree with uncommitted changes)")
		}
	})
}

func TestHandleBashCommit(t *testing.T) {
	// Skip if not in a git repo
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("auto-resets baseline on git commit", func(t *testing.T) {
		// Create a temp git repo for testing
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		// Save and restore current dir
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Create session with old baseline
		sessionID := "test-bash-commit"
		sess, err := state.New(sessionID, "old-tree-sha", "main", 400)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Score = 100 // Some accumulated score
		if err := sess.Save(); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Create a checkpoint dir in this temp repo
		checkpointDir := filepath.Join(tmpDir, ".git", "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		// Simulate a commit
		input := &HookInput{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &ToolInput{Command: "git commit -m 'test commit'"},
		}

		exitCode := PostToolUse(input)
		// Should return 2 (to ensure stderr reaches Claude)
		if exitCode != 2 {
			t.Errorf("PostToolUse(git commit) = %d, want 2", exitCode)
		}

		// Verify session was reset
		reloaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("Failed to reload session: %v", err)
		}

		// BaselineTree should now be the current HEAD tree
		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		expectedTree := string(output)[:len(output)-1] // trim newline

		if reloaded.BaselineTree != expectedTree {
			t.Errorf("BaselineTree = %q, want %q (HEAD^{tree})", reloaded.BaselineTree, expectedTree)
		}

		// Score should be reset to 0
		if reloaded.Score != 0 {
			t.Errorf("Score = %d, want 0 (reset)", reloaded.Score)
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

		input := &HookInput{
			HookEventName: "PostToolUse",
			ToolName:      "Bash",
			SessionID:     sessionID,
			ToolInput:     &ToolInput{Command: "git status"},
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
		input := &HookInput{
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

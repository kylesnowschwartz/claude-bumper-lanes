package hooks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

func TestStopCumulativeStats(t *testing.T) {
	// Skip if not in a git repo or binary not built
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("uses BaselineTree not PreviousTree for breakdown", func(t *testing.T) {
		// Create a temp git repo
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Get baseline tree SHA
		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		baselineTree := strings.TrimSpace(string(output))

		// Create a file with lots of additions to trip threshold
		testFile := filepath.Join(tmpDir, "big-change.go")
		content := "package main\n\n"
		for i := 0; i < 100; i++ {
			content += "// Line " + string(rune('0'+i%10)) + "\n"
		}
		os.WriteFile(testFile, []byte(content), 0644)

		// Create session with:
		// - BaselineTree = original (before changes)
		// - Score over threshold
		sessionID := "test-stop-cumulative"
		sess, err := state.New(sessionID, baselineTree, "main", 50) // Low threshold
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Score = 100 // Already over 50 threshold
		if err := sess.Save(); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// Create lock dir
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		// Capture stdout/stderr to verify response
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		err = Stop(input)

		w.Close()
		os.Stdout = oldStdout

		// Read the output
		var buf [8192]byte
		n, _ := r.Read(buf[:])
		outputStr := string(buf[:n])

		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		// Parse the JSON response
		var resp StopResponse
		if err := json.Unmarshal([]byte(outputStr), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v\nOutput: %s", err, outputStr)
		}

		// Verify it's a block response
		if resp.Decision != "block" {
			t.Errorf("Decision = %q, want 'block'", resp.Decision)
		}

		// Verify cumulative stats are in the response
		// Key check: total additions should be ~100 lines (from BaselineTree diff)
		if resp.ThresholdData == nil {
			t.Fatal("ThresholdData is nil")
		}

		thresholdData, ok := resp.ThresholdData.(map[string]interface{})
		if !ok {
			t.Fatalf("ThresholdData is not a map, got %T", resp.ThresholdData)
		}

		// Check total additions (new + edit) - our file has ~100 lines
		newAdds := int(thresholdData["new_additions"].(float64))
		editAdds := int(thresholdData["edit_additions"].(float64))
		totalAdds := newAdds + editAdds

		// Should show cumulative additions from BaselineTree, not 0
		if totalAdds < 50 {
			t.Errorf("total additions (new=%d + edit=%d = %d), want >= 50 (cumulative from BaselineTree)", newAdds, editAdds, totalAdds)
		}

		t.Logf("Breakdown shows new=%d, edit=%d, total=%d (cumulative from BaselineTree)", newAdds, editAdds, totalAdds)
	})
}

func TestStopAllowsWhenUnderThreshold(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("allows stop when under threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Get current tree
		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		currentTree := strings.TrimSpace(string(output))

		// Create session with high threshold (won't be exceeded)
		sessionID := "test-stop-under"
		sess, err := state.New(sessionID, currentTree, "main", 1000)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.Score = 50 // Well under 1000
		sess.Save()

		// Set up checkpoint dir
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		err = Stop(input)

		// Should return nil (no blocking response)
		if err != nil {
			t.Errorf("Stop() under threshold should return nil, got error: %v", err)
		}
	})
}

func TestStopAutoRecoveryWhenScoreDrops(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("auto-recovers when stop_triggered=true and score drops below threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		currentTree := strings.TrimSpace(string(output))

		// Create session with:
		// - StopTriggered = true (threshold was previously exceeded)
		// - Score = 50 (now BELOW threshold of 100)
		sessionID := "test-stop-recovery"
		sess, err := state.New(sessionID, currentTree, "main", 100)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.StopTriggered = true
		sess.Score = 50 // Below threshold now
		sess.Save()

		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		// Capture stdout to verify recovery response
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		err = Stop(input)

		w.Close()
		os.Stdout = oldStdout

		var buf [8192]byte
		n, _ := r.Read(buf[:])
		outputStr := string(buf[:n])

		if err != nil {
			t.Errorf("Stop() recovery should return nil (WriteResponse handles output), got error: %v", err)
		}

		// Verify recovery message was sent
		if !strings.Contains(outputStr, "Auto-recovered") {
			t.Errorf("Expected recovery message, got: %s", outputStr)
		}

		// Verify StopTriggered was cleared
		reloaded, _ := state.Load(sessionID)
		if reloaded.StopTriggered {
			t.Error("StopTriggered should be false after auto-recovery")
		}

		// Verify score was updated
		if reloaded.Score != 0 {
			t.Errorf("Score = %d, want 0 (clean tree)", reloaded.Score)
		}
	})

	t.Run("keeps stop_triggered when score still above threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		baselineTree := strings.TrimSpace(string(output))

		// Create a file to ensure score > threshold
		testFile := filepath.Join(tmpDir, "over-threshold.go")
		content := "package main\n\n"
		for i := 0; i < 50; i++ {
			content += "// line\n"
		}
		os.WriteFile(testFile, []byte(content), 0644)

		// Create session with StopTriggered=true and low threshold
		sessionID := "test-stop-still-over"
		sess, err := state.New(sessionID, baselineTree, "main", 30) // Low threshold
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.StopTriggered = true
		sess.Score = 60 // Above threshold
		sess.Save()

		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		Stop(input)

		w.Close()
		os.Stdout = oldStdout

		var buf [8192]byte
		n, _ := r.Read(buf[:])
		outputStr := string(buf[:n])

		// Should output threshold exceeded message (not recovery)
		if strings.Contains(outputStr, "Auto-recovered") {
			t.Errorf("Should NOT show recovery message when still over threshold: %s", outputStr)
		}

		if !strings.Contains(outputStr, "threshold exceeded") {
			t.Errorf("Expected 'threshold exceeded' message, got: %s", outputStr)
		}

		// Verify StopTriggered remains true
		reloaded, _ := state.Load(sessionID)
		if !reloaded.StopTriggered {
			t.Error("StopTriggered should remain true when score still above threshold")
		}
	})

	t.Run("clears stop_triggered when score equals threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		currentTree := strings.TrimSpace(string(output))

		// Session with score EXACTLY at threshold
		sessionID := "test-stop-exact"
		sess, err := state.New(sessionID, currentTree, "main", 100)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sess.StopTriggered = true
		sess.Score = 100 // Exactly at threshold
		sess.Save()

		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		checkpointDir := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints")
		os.MkdirAll(checkpointDir, 0755)

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		Stop(input)

		// Verify StopTriggered was cleared (score <= threshold)
		reloaded, _ := state.Load(sessionID)
		if reloaded.StopTriggered {
			t.Error("StopTriggered should be false when score equals threshold")
		}
	})
}

func TestStopSkipsWhenStopHookActive(t *testing.T) {
	t.Run("skips when StopHookActive is true", func(t *testing.T) {
		input := &HookInput{
			SessionID:      "any",
			HookEventName:  "Stop",
			StopHookActive: true, // Already in a Stop hook
		}

		err := Stop(input)

		// Should return nil to prevent infinite loop
		if err != nil {
			t.Errorf("Stop() with StopHookActive=true should return nil, got: %v", err)
		}
	})
}

// TestEndToEndThresholdDecision verifies the full flow: file changes → score → block/allow.
// This catches regressions where scoring works but decision logic breaks.
func TestEndToEndThresholdDecision(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	t.Run("blocks when score exceeds threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		// Get baseline
		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		baselineTree := strings.TrimSpace(string(output))

		// Create file with enough lines to exceed low threshold
		testFile := filepath.Join(tmpDir, "large.go")
		var content strings.Builder
		content.WriteString("package main\n")
		for i := 0; i < 50; i++ {
			content.WriteString("// line\n")
		}
		os.WriteFile(testFile, []byte(content.String()), 0644)

		// Create session with LOW threshold (easy to exceed)
		sessionID := "test-e2e-block"
		sess, _ := state.New(sessionID, baselineTree, "main", 30) // 30 pts = easy to exceed
		sess.Score = 50                                           // Already over
		sess.Save()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		Stop(input)

		w.Close()
		os.Stdout = oldStdout

		var buf [8192]byte
		n, _ := r.Read(buf[:])
		outputStr := string(buf[:n])

		// Should output a block response
		if !strings.Contains(outputStr, `"decision":"block"`) {
			t.Errorf("Expected block decision, got: %s", outputStr)
		}
	})

	t.Run("allows when score under threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupTempGitRepo(t, tmpDir)

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
		output, _ := cmd.Output()
		currentTree := strings.TrimSpace(string(output))

		// Session with HIGH threshold and LOW score
		sessionID := "test-e2e-allow"
		sess, _ := state.New(sessionID, currentTree, "main", 1000)
		sess.Score = 10 // Way under 1000
		sess.Save()

		input := &HookInput{
			SessionID:      sessionID,
			HookEventName:  "Stop",
			StopHookActive: false,
		}

		err := Stop(input)

		// Should return nil (no block)
		if err != nil {
			t.Errorf("Expected nil (allow), got error: %v", err)
		}
	})
}

// TestSessionStateConsistencyAcrossHooks verifies PostToolUse saves state that Stop can read.
// Catches: forgotten Save(), format changes, path mismatches.
func TestSessionStateConsistencyAcrossHooks(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
	output, _ := cmd.Output()
	baselineTree := strings.TrimSpace(string(output))

	sessionID := "test-consistency"

	t.Run("state survives PostToolUse → Stop roundtrip", func(t *testing.T) {
		// Create initial state (simulating SessionStart)
		sess, err := state.New(sessionID, baselineTree, "main", 400)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.Save()

		// Simulate PostToolUse updating state
		loaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("state.Load() error = %v", err)
		}
		loaded.SetScore(150)
		loaded.SetViewMode("sparkline-tree")
		loaded.Save()

		// Simulate Stop reading state
		reloaded, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("state.Load() after update error = %v", err)
		}

		// Verify all fields survived
		if reloaded.Score != 150 {
			t.Errorf("Score = %d, want 150", reloaded.Score)
		}
		if reloaded.GetViewMode() != "sparkline-tree" {
			t.Errorf("ViewMode = %q, want %q", reloaded.GetViewMode(), "sparkline-tree")
		}
		if reloaded.BaselineTree != baselineTree {
			t.Errorf("BaselineTree changed unexpectedly")
		}
		if reloaded.ThresholdLimit != 400 {
			t.Errorf("ThresholdLimit = %d, want 400", reloaded.ThresholdLimit)
		}
	})

	t.Run("state file is valid JSON", func(t *testing.T) {
		// Find the state file
		gitDir, _ := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
		statePath := filepath.Join(strings.TrimSpace(string(gitDir)), "bumper-checkpoints", "session-"+sessionID)

		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		// Should be valid JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Errorf("State file is not valid JSON: %v\nContent: %s", err, string(data))
		}

		// Should have required fields
		requiredFields := []string{"session_id", "baseline_tree", "threshold_limit"}
		for _, field := range requiredFields {
			if _, ok := parsed[field]; !ok {
				t.Errorf("State file missing required field: %s", field)
			}
		}
	})
}

// TestScoreDecreasesWhenFileDeleted verifies fresh-from-baseline scoring behavior.
// Regression test: Score must decrease when user deletes a file, not stay constant.
// This was a bug when using incremental accumulation instead of fresh calculation.
func TestScoreDecreasesWhenFileDeleted(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Get baseline tree SHA
	cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
	output, _ := cmd.Output()
	baselineTree := strings.TrimSpace(string(output))

	// Create session
	sessionID := "test-score-decrease"
	sess, _ := state.New(sessionID, baselineTree, "main", 1000)
	sess.Save()

	// Create a file - score should increase
	testFile := filepath.Join(tmpDir, "test-file.go")
	content := "package main\n\n// This file adds 50 lines\n"
	for i := 0; i < 47; i++ {
		content += "// line\n"
	}
	os.WriteFile(testFile, []byte(content), 0644)

	// Get stats and calculate score after adding file
	stats := getStatsJSON(baselineTree)
	if stats == nil {
		t.Fatal("Failed to get stats after adding file")
	}
	scoreAfterAdd := scoring.Calculate(stats).Score
	t.Logf("Score after adding file: %d", scoreAfterAdd)

	if scoreAfterAdd == 0 {
		t.Error("Score should be > 0 after adding file")
	}

	// Delete the file - score should decrease
	os.Remove(testFile)

	// Get stats and calculate score after deleting file
	stats = getStatsJSON(baselineTree)
	if stats == nil {
		t.Fatal("Failed to get stats after deleting file")
	}
	scoreAfterDelete := scoring.Calculate(stats).Score
	t.Logf("Score after deleting file: %d", scoreAfterDelete)

	// Key assertion: Score MUST be lower after delete
	if scoreAfterDelete >= scoreAfterAdd {
		t.Errorf("Score should decrease after file deletion: before=%d, after=%d",
			scoreAfterAdd, scoreAfterDelete)
	}

	// Score should return to 0 (or near 0) since we're back to baseline state
	if scoreAfterDelete != 0 {
		t.Errorf("Score should be 0 after deleting all added content, got %d", scoreAfterDelete)
	}
}

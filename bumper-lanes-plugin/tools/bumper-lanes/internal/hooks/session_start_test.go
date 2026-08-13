package hooks

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/hookio"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// TestSessionStartPreservesStateOnCompactAndResume verifies that SessionStart
// events reusing an existing session id (compaction, resume) keep the baseline
// and score instead of re-baselining, and inject a budget recap.
func TestSessionStartPreservesStateOnCompactAndResume(t *testing.T) {
	// Isolate HOME: SessionStart's statusline setup must not see the real
	// ~/.claude (belt to the isTestProcess guard's suspenders).
	t.Setenv("HOME", t.TempDir())

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	os.WriteFile("initial.txt", []byte("initial\n"), 0644)
	exec.Command("git", "add", "initial.txt").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	sessionID := "test-preserve-state"
	baseline, _ := git.CaptureTree()

	seedState := func() {
		sess, err := state.New(sessionID, baseline, "main", 600)
		if err != nil {
			t.Fatalf("state.New: %v", err)
		}
		sess.SetScore(400)
		sess.SetStopTriggered(true)
		if err := sess.Save(); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	runSessionStart := func(source string) (int, string) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		exitCode := SessionStart(&hookio.Input{
			SessionID:     sessionID,
			HookEventName: "SessionStart",
			Source:        source,
		})
		w.Close()
		os.Stdout = oldStdout
		out, _ := io.ReadAll(r)
		return exitCode, string(out)
	}

	for _, source := range []string{"compact", "resume"} {
		t.Run(source+" preserves state and injects recap", func(t *testing.T) {
			seedState()
			exitCode, out := runSessionStart(source)
			if exitCode != 0 {
				t.Errorf("SessionStart(%s) = %d, want 0", source, exitCode)
			}

			sess, err := state.Load(sessionID)
			if err != nil {
				t.Fatalf("load after %s: %v", source, err)
			}
			if sess.BaselineTree != baseline {
				t.Errorf("baseline re-captured on %s: %s != %s", source, sess.BaselineTree, baseline)
			}
			if sess.Score != 400 {
				t.Errorf("score = %d after %s, want 400 (preserved)", sess.Score, source)
			}
			if !sess.StopTriggered {
				t.Errorf("stop_triggered cleared on %s, want preserved", source)
			}
			if !strings.Contains(out, "additionalContext") || !strings.Contains(out, "200/600 review-budget pts remain") {
				t.Errorf("recap missing from output on %s: %q", source, out)
			}
		})
	}

	t.Run("startup re-baselines as before", func(t *testing.T) {
		seedState()
		exitCode, _ := runSessionStart("startup")
		if exitCode != 0 && exitCode != 1 {
			t.Errorf("SessionStart(startup) = %d, want 0 or 1", exitCode)
		}
		sess, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("load after startup: %v", err)
		}
		if sess.Score != 0 {
			t.Errorf("score = %d after startup, want 0 (fresh baseline)", sess.Score)
		}
	})

	t.Run("compact without existing state falls through to fresh baseline", func(t *testing.T) {
		state.Delete(sessionID)
		exitCode, _ := runSessionStart("compact")
		if exitCode != 0 && exitCode != 1 {
			t.Errorf("SessionStart(compact, no state) = %d, want 0 or 1", exitCode)
		}
		if _, err := state.Load(sessionID); err != nil {
			t.Errorf("no state created on fall-through: %v", err)
		}
	})
}

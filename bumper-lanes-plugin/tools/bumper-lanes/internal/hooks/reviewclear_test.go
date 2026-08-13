package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/git"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// lastEventEntry reads the last line of the repo's events.jsonl relative to
// the current working directory.
func lastEventEntry(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".git", "bumper-checkpoints", "events.jsonl"))
	if err != nil {
		t.Fatalf("reading events.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("unmarshal last event: %v", err)
	}
	return entry
}

// setupReviewRepo creates a temp repo with on_trip: review configured and a
// tripped session, returning the session id.
func setupReviewRepo(t *testing.T, configJSON string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate global config

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(tmpDir)

	os.WriteFile(".bumper-lanes.json", []byte(configJSON), 0644)

	sessionID := "review-clear-test"
	baseline, err := git.CaptureTree()
	if err != nil {
		t.Fatalf("CaptureTree: %v", err)
	}
	sess, err := state.New(sessionID, baseline, "main", 100)
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	sess.SetScore(150)
	sess.SetStopTriggered(true)
	if err := sess.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	return sessionID
}

func TestReviewClear(t *testing.T) {
	t.Run("clears a tripped session and arms the loop guard", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review"}`)

		if err := ReviewClear(sessionID); err != nil {
			t.Fatalf("ReviewClear: %v", err)
		}

		sess, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("load after clear: %v", err)
		}
		if sess.Score != 0 || sess.StopTriggered {
			t.Errorf("expected cleared state, got score=%d stop=%v", sess.Score, sess.StopTriggered)
		}
		if sess.AutoReviews != 1 {
			t.Errorf("AutoReviews = %d, want 1", sess.AutoReviews)
		}

		// Second self-review this cycle is refused.
		sess.SetScore(150)
		sess.SetStopTriggered(true)
		sess.Save()
		err = ReviewClear(sessionID)
		if err == nil || !strings.Contains(err.Error(), "limit reached") {
			t.Errorf("second clear should hit the loop guard, got: %v", err)
		}
	})

	t.Run("max_auto_reviews -1 allows unlimited clears", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review", "max_auto_reviews": -1}`)
		for i := 0; i < 3; i++ {
			if err := ReviewClear(sessionID); err != nil {
				t.Fatalf("clear %d should be unlimited, got: %v", i+1, err)
			}
			sess, _ := state.Load(sessionID)
			sess.SetScore(150)
			sess.SetStopTriggered(true)
			sess.Save()
		}
	})

	t.Run("max_auto_reviews N allows N clears", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review", "max_auto_reviews": 2}`)
		for i := 0; i < 2; i++ {
			if err := ReviewClear(sessionID); err != nil {
				t.Fatalf("clear %d of 2 should pass, got: %v", i+1, err)
			}
			sess, _ := state.Load(sessionID)
			sess.SetScore(150)
			sess.SetStopTriggered(true)
			sess.Save()
		}
		err := ReviewClear(sessionID)
		if err == nil || !strings.Contains(err.Error(), "limit reached") {
			t.Errorf("third clear should hit the limit of 2, got: %v", err)
		}
	})

	t.Run("stamped policy beats file config for Bash-invoked clears", func(t *testing.T) {
		// File config says block; the hook-stamped policy says review.
		// review-clear must honor the stamp (Bash invocations can't see
		// plugin userConfig env vars, so the stamp is authoritative).
		sessionID := setupReviewRepo(t, `{}`)
		sess, _ := state.Load(sessionID)
		sess.Policy = &state.ReviewPolicy{OnTrip: "review", MaxAutoReviews: 1, ReviewCommand: "/code-review"}
		sess.Save()

		if err := ReviewClear(sessionID); err != nil {
			t.Errorf("stamped review policy should allow the clear, got: %v", err)
		}
	})

	t.Run("max_auto_reviews 0 refuses every clear", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review", "max_auto_reviews": 0}`)
		err := ReviewClear(sessionID)
		if err == nil || !strings.Contains(err.Error(), "limit reached") {
			t.Errorf("clear with limit 0 should refuse, got: %v", err)
		}
	})

	t.Run("refuses when policy is block", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{}`)
		err := ReviewClear(sessionID)
		if err == nil || !strings.Contains(err.Error(), "on_trip") {
			t.Errorf("expected policy refusal, got: %v", err)
		}
	})

	t.Run("clears a partial run: nonzero score, not tripped", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review"}`)
		sess, _ := state.Load(sessionID)
		sess.SetStopTriggered(false)
		sess.Save()

		if err := ReviewClear(sessionID); err != nil {
			t.Fatalf("ReviewClear on partial run: %v", err)
		}

		sess, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("load after clear: %v", err)
		}
		if sess.Score != 0 || sess.StopTriggered {
			t.Errorf("expected cleared state, got score=%d stop=%v", sess.Score, sess.StopTriggered)
		}
		if sess.AutoReviews != 1 {
			t.Errorf("AutoReviews = %d, want 1 (partial-run clears count as self-reviews)", sess.AutoReviews)
		}

		entry := lastEventEntry(t)
		if entry["cause"] != "review" {
			t.Errorf("event cause = %v, want review", entry["cause"])
		}
		if entry["score"].(float64) != 150 {
			t.Errorf("event score = %v, want 150 (pre-clear score)", entry["score"])
		}
	})

	t.Run("fresh clear at score 0 is a no-op notice, not an error", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review"}`)
		sess, _ := state.Load(sessionID)
		sess.SetScore(0)
		sess.SetStopTriggered(false)
		sess.Save()

		if err := ReviewClear(sessionID); err != nil {
			t.Errorf("fresh clear should exit 0 with a notice, got error: %v", err)
		}

		sess, _ = state.Load(sessionID)
		if sess.AutoReviews != 0 {
			t.Errorf("AutoReviews after no-op clear = %d, want 0 (nothing was cleared)", sess.AutoReviews)
		}
	})

	t.Run("tripwire carve-out is configurable", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review", "tripwires_block_auto_review": true}`)
		sess, _ := state.Load(sessionID)
		sess.AddTripwires([]string{"go.mod"})
		sess.Save()
		err := ReviewClear(sessionID)
		if err == nil || !strings.Contains(err.Error(), "tripwires") {
			t.Errorf("expected tripwire refusal, got: %v", err)
		}
	})

	t.Run("tripwires do not block by default", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review"}`)
		sess, _ := state.Load(sessionID)
		sess.AddTripwires([]string{"go.mod"})
		sess.Save()
		if err := ReviewClear(sessionID); err != nil {
			t.Errorf("tripwires should not block by default, got: %v", err)
		}
	})

	t.Run("human reset re-arms self-review", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review"}`)
		if err := ReviewClear(sessionID); err != nil {
			t.Fatalf("first clear: %v", err)
		}
		sess, _ := state.Load(sessionID)
		tree, _ := git.CaptureTree()
		sess.ResetBaseline(tree, "main", "") // commit/branch/manual resets route here
		sess.Save()

		sess, _ = state.Load(sessionID)
		if sess.AutoReviews != 0 {
			t.Errorf("AutoReviews after human-visible reset = %d, want 0", sess.AutoReviews)
		}
	})
}

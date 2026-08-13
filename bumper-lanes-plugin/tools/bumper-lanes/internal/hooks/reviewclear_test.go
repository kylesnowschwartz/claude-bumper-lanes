package hooks

import (
	"os"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

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
	baseline, err := CaptureTree()
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

	t.Run("refuses when not tripped", func(t *testing.T) {
		sessionID := setupReviewRepo(t, `{"on_trip": "review"}`)
		sess, _ := state.Load(sessionID)
		sess.SetStopTriggered(false)
		sess.Save()
		err := ReviewClear(sessionID)
		if err == nil || !strings.Contains(err.Error(), "not tripped") {
			t.Errorf("expected not-tripped refusal, got: %v", err)
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
		tree, _ := CaptureTree()
		sess.ResetBaseline(tree, "main", "") // commit/branch/manual resets route here
		sess.Save()

		sess, _ = state.Load(sessionID)
		if sess.AutoReviews != 0 {
			t.Errorf("AutoReviews after human-visible reset = %d, want 0", sess.AutoReviews)
		}
	})
}

package hooks

import (
	"os"
	"os/exec"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/logging"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// setupRebaseRepo creates a repo with a session baselined at the current
// HEAD, with one uncommitted agent file already counted in the baseline gap.
func setupRebaseRepo(t *testing.T, configJSON string) *state.SessionState {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(tmpDir)
	if configJSON != "" {
		os.WriteFile(".bumper-lanes.json", []byte(configJSON), 0644)
	}

	baseline, err := CaptureTree()
	if err != nil {
		t.Fatalf("CaptureTree: %v", err)
	}
	sess, err := state.New("rebase-test", baseline, "main", 100)
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	sess.BaselineHead = GetHeadCommit()
	// Uncommitted agent work: must still count after any rebase.
	os.WriteFile("agent-work.txt", []byte("uncommitted change\n"), 0644)
	sess.SetScore(1)
	if err := sess.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	return sess
}

// commitUpstreamFile simulates a pull: a commit lands that the session's
// own commit flow never saw.
func commitUpstreamFile(t *testing.T, name string) {
	t.Helper()
	os.WriteFile(name, []byte("upstream line 1\nupstream line 2\n"), 0644)
	exec.Command("git", "add", name).Run()
	if err := exec.Command("git", "commit", "-q", "-m", "upstream").Run(); err != nil {
		t.Fatalf("upstream commit: %v", err)
	}
}

func TestMaybeRebaseBaseline(t *testing.T) {
	log := logging.New("rebase-test", "test")

	t.Run("forgives upstream commits, keeps uncommitted work counted", func(t *testing.T) {
		sess := setupRebaseRepo(t, "")
		commitUpstreamFile(t, "upstream.txt")

		if !maybeRebaseBaseline(sess, log) {
			t.Fatal("expected the baseline to rebase over the upstream commit")
		}
		stats := getStatsJSON(sess.BaselineTree)
		if stats == nil {
			t.Fatal("no stats after rebase")
		}
		var paths []string
		for _, f := range stats.Files {
			paths = append(paths, f.Path)
		}
		if len(paths) != 1 || paths[0] != "agent-work.txt" {
			t.Errorf("post-rebase diff = %v, want only agent-work.txt", paths)
		}
		if sess.BaselineHead != GetHeadCommit() {
			t.Errorf("BaselineHead not advanced to current HEAD")
		}
	})

	t.Run("no-op when HEAD has not moved", func(t *testing.T) {
		sess := setupRebaseRepo(t, "")
		if maybeRebaseBaseline(sess, log) {
			t.Error("baseline moved with HEAD unchanged")
		}
	})

	t.Run("strict reset policies do not auto-rebase", func(t *testing.T) {
		sess := setupRebaseRepo(t, `{"reset_on": "human"}`)
		commitUpstreamFile(t, "upstream.txt")
		if maybeRebaseBaseline(sess, log) {
			t.Error("baseline moved under reset_on: human")
		}
		if note := staleBaselineNote(sess); note == "" {
			t.Error("stale baseline should be named in the trip packet")
		}
	})

	t.Run("bumper-reset anchors the baseline head", func(t *testing.T) {
		sess := setupRebaseRepo(t, "")
		commitUpstreamFile(t, "upstream.txt")
		handleReset(sess.SessionID)
		reloaded, err := state.Load(sess.SessionID)
		if err != nil {
			t.Fatalf("load after reset: %v", err)
		}
		if reloaded.BaselineHead != GetHeadCommit() {
			t.Errorf("BaselineHead after /bumper-reset = %q, want current HEAD", reloaded.BaselineHead)
		}
	})

	t.Run("none sentinel adopts like legacy state", func(t *testing.T) {
		sess := setupRebaseRepo(t, "")
		sess.BaselineHead = "none"
		if maybeRebaseBaseline(sess, log) {
			t.Error("none sentinel should adopt, not rebase")
		}
		if sess.BaselineHead != GetHeadCommit() {
			t.Error("none sentinel did not adopt current HEAD")
		}
	})

	t.Run("legacy state adopts current HEAD without rebasing", func(t *testing.T) {
		sess := setupRebaseRepo(t, "")
		sess.BaselineHead = ""
		if maybeRebaseBaseline(sess, log) {
			t.Error("legacy state should adopt, not rebase")
		}
		if sess.BaselineHead != GetHeadCommit() {
			t.Error("legacy state did not adopt current HEAD")
		}
	})

	t.Run("conflicting upstream change fails open", func(t *testing.T) {
		sess := setupRebaseRepo(t, "")
		// A conflict needs one path changed on both sides of the 3-way:
		// in the baseline (an edit that was uncommitted at capture time)
		// and in the upstream commits.
		os.WriteFile("shared.txt", []byte("base\n"), 0644)
		exec.Command("git", "add", "shared.txt").Run()
		exec.Command("git", "commit", "-q", "-m", "shared base").Run()
		os.WriteFile("shared.txt", []byte("edit at capture time\n"), 0644)
		baseline, _ := CaptureTree()
		sess.BaselineTree = baseline
		sess.BaselineHead = GetHeadCommit()
		sess.Save()

		os.WriteFile("shared.txt", []byte("upstream edit\n"), 0644)
		exec.Command("git", "add", "shared.txt").Run()
		exec.Command("git", "commit", "-q", "-m", "upstream shared").Run()

		oldTree := sess.BaselineTree
		if maybeRebaseBaseline(sess, log) {
			t.Fatal("conflicting merge should fail open, not rebase")
		}
		if sess.BaselineTree != oldTree {
			t.Errorf("baseline changed on a failed merge")
		}
		if note := staleBaselineNote(sess); note == "" {
			t.Error("failed rebase should leave a stale-baseline note")
		}
	})
}

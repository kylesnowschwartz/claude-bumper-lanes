package hooks

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

func TestMatchTripwirePath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{".github/workflows/**", ".github/workflows/ci.yml", true},
		{".github/workflows/**", ".github/workflows/deploy/prod.yml", true},
		{".github/workflows/**", ".github/dependabot.yml", false},
		{"go.mod", "go.mod", true},
		{"go.mod", "tools/api/go.mod", true}, // basename match for slashless patterns
		{"go.mod", "go.sum", false},
		{"**/hooks.json", "plugin/hooks/hooks.json", true},
		{"**/hooks.json", "hooks.json", true},
		{"**/migrations/**", "app/db/migrations/0001_init.sql", true},
		{"**/migrations/**", "app/db/schema.sql", false},
		{".claude/settings*.json", ".claude/settings.json", true},
		{".claude/settings*.json", ".claude/settings.local.json", true},
		{".claude/settings*.json", ".claude/skills/x.json", false},
		{"db/migrate/**", "db/migrate/20260811_add.rb", true},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"~"+tt.path, func(t *testing.T) {
			if got := matchTripwirePath(tt.pattern, tt.path); got != tt.want {
				t.Errorf("matchTripwirePath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

// TestTripwireDetection verifies the manual-check requirement from the
// audit: a t.Skip insertion at a tiny score must surface immediately.
func TestTripwireDetection(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

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
	baseline, _ := CaptureTree()
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

	input := &HookInput{
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
	if !IsGitRepo() {
		t.Skip("Not in a git repo")
	}

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)
	os.WriteFile(".bumper-lanes.json", []byte(`{"tripwire_paths": [], "tripwire_patterns": []}`), 0644)

	sessionID := "test-tripwires-off"
	baseline, _ := CaptureTree()
	sess, _ := state.New(sessionID, baseline, "main", 600)
	sess.Save()

	os.MkdirAll(".github/workflows", 0755)
	os.WriteFile(".github/workflows/ci.yml", []byte("on: push\n"), 0644)

	PostToolUse(&HookInput{HookEventName: "PostToolUse", ToolName: "Edit", SessionID: sessionID})

	reloaded, _ := state.Load(sessionID)
	if len(reloaded.Tripwires) != 0 {
		t.Errorf("Tripwires = %v with tripwires disabled, want empty", reloaded.Tripwires)
	}
}

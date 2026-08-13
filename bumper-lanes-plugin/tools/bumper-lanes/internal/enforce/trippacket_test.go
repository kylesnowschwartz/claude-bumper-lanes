package enforce

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

func packetFixture() (*state.SessionState, *scoring.WeightedScore, *diff.StatsJSON) {
	stats := &diff.StatsJSON{
		Files: []diff.FileStatJSON{
			{Path: "internal/hooks/stop.go", Adds: 200, Dels: 40},
			{Path: "internal/hooks/packet.go", Adds: 150, Dels: 0, New: true},
			{Path: "internal/scoring/scoring.go", Adds: 100, Dels: 10},
			{Path: "go.sum", Adds: 50, Dels: 50},
			{Path: "README.md", Adds: 30, Dels: 5},
		},
		Totals: diff.TotalsJSON{Adds: 530, Dels: 105, FileCount: 5},
	}
	result := scoring.Calculate(stats)
	sess := &state.SessionState{
		SessionID:      "test",
		ThresholdLimit: 600,
		Tripwires:      []string{"go.mod"},
	}
	return sess, result, stats
}

func TestBuildTripPacket_Golden(t *testing.T) {
	sess, result, stats := packetFixture()
	got := BuildTripPacket(sess, result, stats, HumanNextMove)

	want := `
⚠️  Bumper lanes: review budget tripped - 579/600 pts (96%)

Tripwires (review these first): go.mod

Changes by module (weighted points):
 410pts  internal/hooks/ (2 files)
 130pts  internal/scoring/ (1 file)
  39pts  (root) (1 file)

New files (new surface area, review the boundaries):
- internal/hooks/packet.go

Give an account of what changed at the module level and why. State whether
the shape of this change matches what was asked for. State what you verified
and how. Then offer the user: (a) review now, (b) /bumper-reset if already
reviewed, or (c) split the remaining work into smaller increments.

Files (additions-ranked):
5 files changed, +530 -105
+200 -40 internal/hooks/stop.go
+150 -0 internal/hooks/packet.go [new]
+100 -10 internal/scoring/scoring.go
+50 -50 go.sum
+30 -5 README.md
`
	if got != want {
		t.Errorf("trip packet mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestBuildTripPacket_NoTripwiresNoNewFiles(t *testing.T) {
	stats := &diff.StatsJSON{
		Files:  []diff.FileStatJSON{{Path: "main.go", Adds: 500, Dels: 0}},
		Totals: diff.TotalsJSON{Adds: 500, Dels: 0, FileCount: 1},
	}
	result := scoring.Calculate(stats)
	sess := &state.SessionState{SessionID: "test", ThresholdLimit: 600}

	got := BuildTripPacket(sess, result, stats, HumanNextMove)
	if strings.Contains(got, "Tripwires") {
		t.Errorf("packet should omit tripwire section when none hit:\n%s", got)
	}
	if strings.Contains(got, "New files") {
		t.Errorf("packet should omit new-files section when none:\n%s", got)
	}
	if !strings.Contains(got, "650/600 pts") {
		t.Errorf("packet missing score line:\n%s", got)
	}
}

func TestReviewNextMove(t *testing.T) {
	got := ReviewNextMove("/code-review")
	for _, want := range []string{"on_trip: review", "/code-review", "review-clear", "next increment"} {
		if !strings.Contains(got, want) {
			t.Errorf("ReviewNextMove missing %q:\n%s", want, got)
		}
	}
}

func TestTripNotification(t *testing.T) {
	got := TripNotification(659, 600)
	want := "\x1b]9;bumper-lanes: review budget tripped (659/600 pts)\x07"
	if got != want {
		t.Errorf("TripNotification = %q, want %q", got, want)
	}
}

func TestByModule(t *testing.T) {
	_, _, stats := packetFixture()
	modules := scoring.ByModule(stats)

	// go.sum is generated: excluded entirely.
	for _, m := range modules {
		if m.Module == "(root)" && m.Files != 1 {
			t.Errorf("(root) should have 1 file (README.md only, go.sum generated), got %d", m.Files)
		}
	}
	if len(modules) != 3 {
		t.Fatalf("modules = %d, want 3", len(modules))
	}
	// Points-ranked: hooks (200*1.3 + 150*1.0 = 410) first.
	if modules[0].Module != "internal/hooks/" || modules[0].Points != 410 {
		t.Errorf("top module = %s %dpts, want internal/hooks/ 410pts", modules[0].Module, modules[0].Points)
	}
}

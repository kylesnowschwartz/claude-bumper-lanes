package hooks

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/scoring"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/diff-viz/v2/diff"
	"github.com/kylesnowschwartz/diff-viz/v2/render"
)

// tripNotification is the OSC 9 desktop-notification escape sequence for a
// fresh trip, so an unattended session isn't silently blocked for hours.
func tripNotification(score, limit int) string {
	return fmt.Sprintf("\x1b]9;bumper-lanes: review budget tripped (%d/%d pts)\x07", score, limit)
}

// buildTripPacket assembles the trip-time review packet: the meter's ground
// truth presented at the altitude engineers review (modules and decisions),
// a scripted next move for the model, and a plain-rendered file appendix.
// The meter holds facts the model's own summary omits; pairing them is what
// turns the stop sign into the review. nextMove is one of humanNextMove,
// reviewNextMove(...), or humanNextMove+escalationNote.
func buildTripPacket(sess *state.SessionState, result *scoring.WeightedScore, stats *diff.StatsJSON, nextMove string) string {
	var b strings.Builder
	pct := (result.Score * 100) / sess.ThresholdLimit

	fmt.Fprintf(&b, "\n⚠️  Bumper lanes: review budget tripped - %d/%d pts (%d%%)\n",
		result.Score, sess.ThresholdLimit, pct)

	if len(sess.Tripwires) > 0 {
		fmt.Fprintf(&b, "\nTripwires (review these first): %s\n", strings.Join(sess.Tripwires, ", "))
	}

	if modules := scoring.ByModule(stats); len(modules) > 0 {
		fmt.Fprintf(&b, "\nChanges by module (weighted points):\n")
		for _, m := range modules {
			fileWord := "files"
			if m.Files == 1 {
				fileWord = "file"
			}
			fmt.Fprintf(&b, "%4dpts  %s (%d %s)\n", m.Points, m.Module, m.Files, fileWord)
		}
		if result.ScatterPenalty > 0 {
			fmt.Fprintf(&b, "%4dpts  scatter penalty (%d files touched)\n", result.ScatterPenalty, result.FilesTouched)
		}
	}

	if newFiles := newFilePaths(stats); len(newFiles) > 0 {
		fmt.Fprintf(&b, "\nNew files (new surface area, review the boundaries):\n")
		for _, p := range newFiles {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}

	b.WriteString(nextMove)

	b.WriteString("\nFiles (additions-ranked):\n")
	b.WriteString(renderPlainAppendix(stats))

	return b.String()
}

// humanNextMove is the scripted move for the default (block) trip policy.
const humanNextMove = `
Give an account of what changed at the module level and why. State whether
the shape of this change matches what was asked for. State what you verified
and how. Then offer the user: (a) review now, (b) /bumper-reset if already
reviewed, or (c) split the remaining work into smaller increments.
`

// reviewNextMove scripts the self-review flow (on_trip: review). The clear
// happens BEFORE implementing findings so the fixes are metered as the next
// increment rather than riding free on the reviewed one.
func reviewNextMove(reviewCommand string) string {
	return fmt.Sprintf(`
Self-review is enabled (on_trip: review). Do this now, in order:
1. Review this increment with %s, scoped to the files listed below
   (the diff against the review baseline - /bumper-diff shows it).
2. When the review has run, clear the breaker:
   %s review-clear
3. Implement the review findings as the next increment (fresh budget),
   then continue the original task.
`, reviewCommand, getBumperLanesBinPath())
}

// escalationNote explains why a review-policy trip is showing the human
// packet anyway.
const escalationNote = `
Note: the self-review limit for this cycle is reached; this trip requires
the user.
`

// newFilePaths lists non-generated new files, in stats order.
func newFilePaths(stats *diff.StatsJSON) []string {
	var paths []string
	for _, f := range stats.Files {
		if f.New && !scoring.IsGenerated(f.Path) {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

// renderPlainAppendix renders the file-level ground truth with diff-viz's
// model-facing plain mode.
func renderPlainAppendix(stats *diff.StatsJSON) string {
	full := &diff.DiffStats{
		TotalAdd:   stats.Totals.Adds,
		TotalDel:   stats.Totals.Dels,
		TotalFiles: stats.Totals.FileCount,
	}
	for _, f := range stats.Files {
		full.Files = append(full.Files, diff.FileStat{
			Path:        f.Path,
			Additions:   f.Adds,
			Deletions:   f.Dels,
			IsBinary:    f.Binary,
			IsUntracked: f.New,
		})
	}
	var buf bytes.Buffer
	render.NewPlainRenderer(&buf).Render(full)
	return buf.String()
}

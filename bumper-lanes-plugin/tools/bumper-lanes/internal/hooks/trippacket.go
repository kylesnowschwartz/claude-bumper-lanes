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
// turns the stop sign into the review.
func buildTripPacket(sess *state.SessionState, result *scoring.WeightedScore, stats *diff.StatsJSON) string {
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

	b.WriteString(`
Give an account of what changed at the module level and why. State whether
the shape of this change matches what was asked for. State what you verified
and how. Then offer the user: (a) review now, (b) /bumper-reset if already
reviewed, or (c) split the remaining work into smaller increments.

Files (additions-ranked):
`)
	b.WriteString(renderPlainAppendix(stats))

	return b.String()
}

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

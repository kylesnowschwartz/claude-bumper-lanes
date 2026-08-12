package hooks

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
	"github.com/kylesnowschwartz/diff-viz/v2/render"
)

// renderBaselineDiff renders the working-tree-vs-baseline diff as an
// indented file tree - the same diff the budget meters, not the HEAD diff.
// Color is off because the output lands in the transcript via a blocked
// prompt. Returns "" when there is nothing to show.
func renderBaselineDiff(baselineTree string) (string, error) {
	currentTree, err := diff.CaptureCurrentTree()
	if err != nil {
		return "", fmt.Errorf("capturing working tree: %w", err)
	}
	stats, _, err := diff.GetTreeDiffStats(baselineTree, currentTree)
	if err != nil {
		return "", fmt.Errorf("diffing against baseline: %w", err)
	}
	if stats.TotalFiles == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	render.NewTreeRenderer(&buf, false).Render(stats)
	rendered := strings.TrimRight(buf.String(), " \t\n\r")
	if rendered == "No changes" {
		return "", nil
	}
	return rendered, nil
}

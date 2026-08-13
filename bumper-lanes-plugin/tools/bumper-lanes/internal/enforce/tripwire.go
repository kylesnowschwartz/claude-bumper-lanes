// Package enforce holds the trip-policy domain: tripwire detection for
// high-risk change classes, and the trip packet presented when the review
// budget trips.
package enforce

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/config"
	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

// Tripwires are zero-threshold lanes: the dangerous edits are small (a CI
// workflow tweak, a new dependency, a skipped test), so a volume meter is
// structurally blind to them. Any hit is surfaced immediately at any score
// and named for the human reviewer.

// matchTripwirePath reports whether a repo-relative path matches a glob
// pattern supporting `*` (within a segment) and `**` (across segments).
// A pattern without a slash also matches against the basename, so "go.mod"
// hits "tools/api/go.mod".
func matchTripwirePath(pattern, path string) bool {
	if !strings.Contains(pattern, "/") {
		base := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			base = path[i+1:]
		}
		return globToRegexp(pattern).MatchString(base)
	}
	return globToRegexp(pattern).MatchString(path)
}

// globToRegexp compiles a glob into an anchored regexp: `**` matches any
// characters including slashes, `*` matches within one path segment.
func globToRegexp(glob string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString(`(.*/)?`)
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(`.*`)
			i++
		case glob[i] == '*':
			b.WriteString(`[^/]*`)
		default:
			b.WriteString(regexp.QuoteMeta(string(glob[i])))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// DetectTripwires returns human-readable descriptions of tripwire hits in
// the current diff: changed files matching tripwire path globs, and added
// lines containing tripwire patterns. stats is the already-computed diff
// from baseline; baselineTree is used for the added-line scan.
func DetectTripwires(stats *diff.StatsJSON, baselineTree string) []string {
	var hits []string

	for _, pattern := range config.LoadTripwirePaths() {
		for _, file := range stats.Files {
			if matchTripwirePath(pattern, file.Path) {
				hits = append(hits, file.Path)
			}
		}
	}

	patterns := config.LoadTripwirePatterns()
	if len(patterns) > 0 {
		for _, hit := range scanAddedLines(baselineTree, patterns) {
			hits = append(hits, hit)
		}
	}

	return hits
}

// scanAddedLines diffs the baseline tree against the working tree and
// returns "pattern (file)" for each tripwire pattern found on an added
// line. Untracked files are not scanned (git diff <tree> covers tracked
// content); a skip added to a brand-new test file is out of scope here.
func scanAddedLines(baselineTree string, patterns []string) []string {
	output, err := exec.Command("git", "diff", "-U0", baselineTree).Output()
	if err != nil {
		return nil // Fail open: no scan beats a blocked edit
	}

	var hits []string
	seen := map[string]bool{}
	currentFile := ""
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		for _, p := range patterns {
			if strings.Contains(line, p) {
				key := fmt.Sprintf("%s (%s)", p, currentFile)
				if !seen[key] {
					seen[key] = true
					hits = append(hits, key)
				}
			}
		}
	}
	return hits
}

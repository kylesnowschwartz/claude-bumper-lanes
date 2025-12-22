// Package render provides diff visualization renderers.
package render

// ANSI color codes for diff visualization.
const (
	ColorDir   = "\033[34m"     // Blue for directories
	ColorFile  = "\033[38;5;8m" // Dark gray for files
	ColorNew   = "\033[33m"     // Yellow for untracked/new
	ColorAdd   = "\033[32m"     // Green for additions
	ColorDel   = "\033[31m"     // Red for deletions
	ColorReset = "\033[0m"      // Reset to default
)

// ColorFunc returns a function that wraps text in ANSI color codes.
// When useColor is false, returns a no-op function.
func ColorFunc(useColor bool) func(string) string {
	if useColor {
		return func(code string) string { return code }
	}
	return func(string) string { return "" }
}

// Separator returns the appropriate separator for output.
// Returns box-drawing character when colors are enabled, ASCII otherwise.
func Separator(useColor bool) string {
	if useColor {
		return " │ "
	}
	return " | "
}

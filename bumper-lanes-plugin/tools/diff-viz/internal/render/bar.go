// Package render provides diff visualization renderers.
package render

import "strings"

// Block characters for bar rendering.
const (
	BlockFull   = "█" // U+2588 Full block (high magnitude)
	BlockMedium = "▓" // U+2593 Dark shade (medium magnitude)
	BlockLight  = "▒" // U+2592 Medium shade (low magnitude)
	BlockEmpty  = "░" // U+2591 Light shade (empty/padding)
)

// Threshold maps a minimum total change count to a bar fill level.
type Threshold struct {
	MinTotal int // Minimum total changes required
	Filled   int // Number of filled blocks
}

// CharLevel maps a minimum total change count to a block character.
type CharLevel struct {
	MinTotal int    // Minimum total changes required
	Char     string // Block character to use
}

// DefaultThresholds maps total changes to bar fill counts.
// Ordered descending so first match wins.
var DefaultThresholds = []Threshold{
	{400, 10}, {300, 9}, {200, 8}, {150, 7}, {100, 6},
	{75, 5}, {50, 4}, {30, 3}, {15, 2}, {0, 1},
}

// DefaultCharLevels maps total changes to block density characters.
// Higher totals get denser blocks for visual emphasis.
var DefaultCharLevels = []CharLevel{
	{200, BlockFull},
	{100, BlockMedium},
	{0, BlockLight},
}

// BarConfig controls bar rendering behavior.
type BarConfig struct {
	Width      int          // Maximum bar width in characters
	Thresholds []Threshold  // Fill level thresholds
	CharLevels []CharLevel  // Block character thresholds
}

// DefaultBarConfig returns a BarConfig with sensible defaults.
func DefaultBarConfig(width int) BarConfig {
	return BarConfig{
		Width:      width,
		Thresholds: DefaultThresholds,
		CharLevels: DefaultCharLevels,
	}
}

// FilledFor returns the number of filled blocks for a given total.
func (c BarConfig) FilledFor(total int) int {
	for _, t := range c.Thresholds {
		if total >= t.MinTotal {
			return min(t.Filled, c.Width)
		}
	}
	return 1
}

// BlockChar returns the appropriate block character based on magnitude.
func (c BarConfig) BlockChar(total int) string {
	for _, l := range c.CharLevels {
		if total >= l.MinTotal {
			return l.Char
		}
	}
	return BlockLight
}

// RatioBar renders a bar split proportionally between additions and deletions.
// Parameters:
//   - add, del: line counts for additions and deletions
//   - filled: number of blocks to fill (from FilledFor or proportional calc)
//   - barWidth: total width including padding
//   - block: the block character to use (from BlockChar)
//   - colorFn: function to apply ANSI color codes (from ColorFunc)
//
// Returns the formatted bar string with green add blocks, red del blocks,
// and empty padding blocks.
func RatioBar(add, del, filled, barWidth int, block string, colorFn func(string) string) string {
	total := add + del
	if total == 0 {
		return strings.Repeat(BlockEmpty, barWidth)
	}

	// Ensure minimum 2 blocks when both add and del exist
	// so we can always show the split
	if add > 0 && del > 0 && filled < 2 {
		filled = 2
	}

	// Cap filled at barWidth
	if filled > barWidth {
		filled = barWidth
	}

	// Split bar into add (green) and del (red) portions
	addBlocks := (add * filled) / total
	delBlocks := filled - addBlocks

	// Ensure at least 1 block for non-zero values
	if add > 0 && addBlocks == 0 {
		addBlocks = 1
		delBlocks = filled - 1
	} else if del > 0 && delBlocks == 0 {
		delBlocks = 1
		addBlocks = filled - 1
	}

	var sb strings.Builder
	if addBlocks > 0 {
		sb.WriteString(colorFn(ColorAdd))
		sb.WriteString(strings.Repeat(block, addBlocks))
		sb.WriteString(colorFn(ColorReset))
	}
	if delBlocks > 0 {
		sb.WriteString(colorFn(ColorDel))
		sb.WriteString(strings.Repeat(block, delBlocks))
		sb.WriteString(colorFn(ColorReset))
	}

	// Pad with empty blocks
	if padding := barWidth - filled; padding > 0 {
		sb.WriteString(strings.Repeat(BlockEmpty, padding))
	}

	return sb.String()
}

// Package-level helpers using defaults for backwards compatibility.
// These match the original function signatures in topn.go.

// filledFromTotal returns the number of filled bar blocks for a given total.
// Uses default thresholds with width 10.
func filledFromTotal(total int) int {
	return DefaultBarConfig(10).FilledFor(total)
}

// blockChar returns the appropriate block character based on magnitude.
// Uses default char levels.
func blockChar(total int) string {
	return DefaultBarConfig(10).BlockChar(total)
}

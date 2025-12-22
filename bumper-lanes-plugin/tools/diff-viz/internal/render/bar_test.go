package render

import (
	"strings"
	"testing"
)

func TestBarConfig_FilledFor(t *testing.T) {
	cfg := DefaultBarConfig(10)

	tests := []struct {
		total int
		want  int
	}{
		{0, 1},     // minimum 1 block
		{14, 1},    // below 15 threshold
		{15, 2},    // at threshold
		{29, 2},    // below next
		{30, 3},
		{50, 4},
		{75, 5},
		{100, 6},
		{150, 7},
		{200, 8},
		{300, 9},
		{400, 10},  // max
		{1000, 10}, // capped at width
	}

	for _, tt := range tests {
		got := cfg.FilledFor(tt.total)
		if got != tt.want {
			t.Errorf("FilledFor(%d) = %d, want %d", tt.total, got, tt.want)
		}
	}
}

func TestBarConfig_FilledFor_CustomWidth(t *testing.T) {
	cfg := DefaultBarConfig(5) // smaller width

	// Should cap at width even if threshold says 10
	got := cfg.FilledFor(500)
	if got != 5 {
		t.Errorf("FilledFor(500) with width=5 = %d, want 5", got)
	}
}

func TestBarConfig_BlockChar(t *testing.T) {
	cfg := DefaultBarConfig(10)

	tests := []struct {
		total int
		want  string
	}{
		{0, BlockLight},   // below 100
		{50, BlockLight},  // below 100
		{99, BlockLight},  // just below
		{100, BlockMedium},
		{199, BlockMedium},
		{200, BlockFull},
		{500, BlockFull},
	}

	for _, tt := range tests {
		got := cfg.BlockChar(tt.total)
		if got != tt.want {
			t.Errorf("BlockChar(%d) = %q, want %q", tt.total, got, tt.want)
		}
	}
}

// noColor returns the string unchanged (no ANSI codes).
func noColor(s string) string { return "" }

// identityColor returns input unchanged for testing color placement.
func identityColor(s string) string { return s }

func TestRatioBar_Empty(t *testing.T) {
	got := RatioBar(0, 0, 5, 10, BlockFull, noColor)
	want := strings.Repeat(BlockEmpty, 10)
	if got != want {
		t.Errorf("RatioBar(0, 0) = %q, want all empty blocks", got)
	}
}

func TestRatioBar_AllAdditions(t *testing.T) {
	got := RatioBar(100, 0, 5, 10, BlockFull, noColor)

	// Should have 5 filled blocks + 5 empty
	filledCount := strings.Count(got, BlockFull)
	emptyCount := strings.Count(got, BlockEmpty)

	if filledCount != 5 {
		t.Errorf("expected 5 filled blocks, got %d", filledCount)
	}
	if emptyCount != 5 {
		t.Errorf("expected 5 empty blocks, got %d", emptyCount)
	}
}

func TestRatioBar_AllDeletions(t *testing.T) {
	got := RatioBar(0, 100, 5, 10, BlockFull, noColor)

	filledCount := strings.Count(got, BlockFull)
	emptyCount := strings.Count(got, BlockEmpty)

	if filledCount != 5 {
		t.Errorf("expected 5 filled blocks, got %d", filledCount)
	}
	if emptyCount != 5 {
		t.Errorf("expected 5 empty blocks, got %d", emptyCount)
	}
}

func TestRatioBar_SplitRatio(t *testing.T) {
	// 50/50 split with 10 filled blocks
	got := RatioBar(50, 50, 10, 10, BlockFull, noColor)

	// All blocks should be filled (10 filled, 0 empty)
	filledCount := strings.Count(got, BlockFull)
	if filledCount != 10 {
		t.Errorf("expected 10 filled blocks for 50/50 split, got %d", filledCount)
	}
}

func TestRatioBar_MinimumTwoBlocks(t *testing.T) {
	// When both add and del exist but filled=1, should bump to 2
	got := RatioBar(10, 10, 1, 10, BlockFull, noColor)

	filledCount := strings.Count(got, BlockFull)
	if filledCount != 2 {
		t.Errorf("expected 2 blocks (min for split), got %d", filledCount)
	}
}

func TestRatioBar_AtLeastOneBlock(t *testing.T) {
	// Even tiny non-zero values get at least 1 block
	tests := []struct {
		name string
		add  int
		del  int
	}{
		{"tiny add", 1, 99},
		{"tiny del", 99, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use identityColor to preserve color codes in output
			got := RatioBar(tt.add, tt.del, 10, 10, BlockFull, identityColor)

			// Both colors should appear
			if !strings.Contains(got, ColorAdd) {
				t.Error("expected green color code for additions")
			}
			if !strings.Contains(got, ColorDel) {
				t.Error("expected red color code for deletions")
			}
		})
	}
}

func TestRatioBar_CapsAtWidth(t *testing.T) {
	// filled > barWidth should be capped
	got := RatioBar(100, 0, 20, 10, BlockFull, noColor)

	totalBlocks := strings.Count(got, BlockFull) + strings.Count(got, BlockEmpty)
	if totalBlocks != 10 {
		t.Errorf("expected 10 total blocks (width cap), got %d", totalBlocks)
	}
}

package scoring

import (
	"testing"

	"github.com/kylewlacy/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name    string
		stats   *diff.DiffStats
		want    WeightedScore
	}{
		{
			name:  "empty stats",
			stats: &diff.DiffStats{},
			want:  WeightedScore{Score: 0},
		},
		{
			name: "single new file (1.0x weight)",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "new.go", Additions: 100, IsUntracked: true},
				},
				TotalAdd:   100,
				TotalFiles: 1,
			},
			// 100 * 10 / 10 = 100
			want: WeightedScore{Score: 100, NewAdditions: 100, FilesTouched: 1},
		},
		{
			name: "single edited file (1.3x weight)",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "edit.go", Additions: 100, IsUntracked: false},
				},
				TotalAdd:   100,
				TotalFiles: 1,
			},
			// 100 * 13 / 10 = 130
			want: WeightedScore{Score: 130, EditAdditions: 100, FilesTouched: 1},
		},
		{
			name: "mixed new and edit",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "new.go", Additions: 50, IsUntracked: true},
					{Path: "edit.go", Additions: 50, IsUntracked: false},
				},
				TotalAdd:   100,
				TotalFiles: 2,
			},
			// (50*10 + 50*13) / 10 = 115
			want: WeightedScore{Score: 115, NewAdditions: 50, EditAdditions: 50, FilesTouched: 2},
		},
		{
			name: "deletions ignored in score",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "refactor.go", Additions: 10, Deletions: 100, IsUntracked: false},
				},
				TotalAdd:   10,
				TotalDel:   100,
				TotalFiles: 1,
			},
			// Only additions counted: 10 * 13 / 10 = 13
			want: WeightedScore{Score: 13, EditAdditions: 10, FilesTouched: 1},
		},
		{
			name: "5 files: no scatter penalty (free tier)",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "a.go", Additions: 10},
					{Path: "b.go", Additions: 10},
					{Path: "c.go", Additions: 10},
					{Path: "d.go", Additions: 10},
					{Path: "e.go", Additions: 10},
				},
				TotalAdd:   50,
				TotalFiles: 5,
			},
			// 50 * 13 / 10 = 65, no scatter
			want: WeightedScore{Score: 65, EditAdditions: 50, FilesTouched: 5},
		},
		{
			name: "6 files: low scatter penalty begins",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "a.go", Additions: 10},
					{Path: "b.go", Additions: 10},
					{Path: "c.go", Additions: 10},
					{Path: "d.go", Additions: 10},
					{Path: "e.go", Additions: 10},
					{Path: "f.go", Additions: 10},
				},
				TotalAdd:   60,
				TotalFiles: 6,
			},
			// 60 * 13 / 10 = 78, scatter = (6-5)*10 = 10
			want: WeightedScore{Score: 88, EditAdditions: 60, FilesTouched: 6, ScatterPenalty: 10},
		},
		{
			name: "10 files: max low scatter",
			stats: &diff.DiffStats{
				Files:      make([]diff.FileStat, 10),
				TotalAdd:   100,
				TotalFiles: 10,
			},
			// scatter = (10-5)*10 = 50
			want: WeightedScore{Score: 50, FilesTouched: 10, ScatterPenalty: 50},
		},
		{
			name: "11 files: high scatter penalty kicks in",
			stats: &diff.DiffStats{
				Files:      make([]diff.FileStat, 11),
				TotalAdd:   0,
				TotalFiles: 11,
			},
			// scatter = (11-5)*30 = 180
			want: WeightedScore{Score: 180, FilesTouched: 11, ScatterPenalty: 180},
		},
		{
			name: "binary files contribute nothing",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "image.png", IsBinary: true},
				},
				TotalFiles: 1,
			},
			want: WeightedScore{Score: 0, FilesTouched: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calculate(tt.stats)
			if got.Score != tt.want.Score {
				t.Errorf("Score = %d, want %d", got.Score, tt.want.Score)
			}
			if got.NewAdditions != tt.want.NewAdditions {
				t.Errorf("NewAdditions = %d, want %d", got.NewAdditions, tt.want.NewAdditions)
			}
			if got.EditAdditions != tt.want.EditAdditions {
				t.Errorf("EditAdditions = %d, want %d", got.EditAdditions, tt.want.EditAdditions)
			}
			if got.FilesTouched != tt.want.FilesTouched {
				t.Errorf("FilesTouched = %d, want %d", got.FilesTouched, tt.want.FilesTouched)
			}
			if got.ScatterPenalty != tt.want.ScatterPenalty {
				t.Errorf("ScatterPenalty = %d, want %d", got.ScatterPenalty, tt.want.ScatterPenalty)
			}
		})
	}
}

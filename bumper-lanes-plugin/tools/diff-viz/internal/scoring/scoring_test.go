package scoring

import (
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/diff-viz/internal/diff"
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
			name: "10 files with additions: max low scatter",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "a.go", Additions: 1}, {Path: "b.go", Additions: 1},
					{Path: "c.go", Additions: 1}, {Path: "d.go", Additions: 1},
					{Path: "e.go", Additions: 1}, {Path: "f.go", Additions: 1},
					{Path: "g.go", Additions: 1}, {Path: "h.go", Additions: 1},
					{Path: "i.go", Additions: 1}, {Path: "j.go", Additions: 1},
				},
				TotalAdd:   10,
				TotalFiles: 10,
			},
			// 10 * 13 / 10 = 13, scatter = (10-5)*10 = 50
			want: WeightedScore{Score: 63, EditAdditions: 10, FilesTouched: 10, ScatterPenalty: 50},
		},
		{
			name: "11 files with additions: high scatter penalty kicks in",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "a.go", Additions: 1}, {Path: "b.go", Additions: 1},
					{Path: "c.go", Additions: 1}, {Path: "d.go", Additions: 1},
					{Path: "e.go", Additions: 1}, {Path: "f.go", Additions: 1},
					{Path: "g.go", Additions: 1}, {Path: "h.go", Additions: 1},
					{Path: "i.go", Additions: 1}, {Path: "j.go", Additions: 1},
					{Path: "k.go", Additions: 1},
				},
				TotalAdd:   11,
				TotalFiles: 11,
			},
			// 11 * 13 / 10 = 14, scatter = (11-5)*30 = 180
			want: WeightedScore{Score: 194, EditAdditions: 11, FilesTouched: 11, ScatterPenalty: 180},
		},
		{
			name: "files with only deletions don't count toward scatter",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "deleted1.go", Additions: 0, Deletions: 100},
					{Path: "deleted2.go", Additions: 0, Deletions: 100},
					{Path: "deleted3.go", Additions: 0, Deletions: 100},
					{Path: "deleted4.go", Additions: 0, Deletions: 100},
					{Path: "deleted5.go", Additions: 0, Deletions: 100},
					{Path: "deleted6.go", Additions: 0, Deletions: 100},
					{Path: "deleted7.go", Additions: 0, Deletions: 100},
				},
				TotalDel:   700,
				TotalFiles: 7,
			},
			// No additions = score 0, scatter 0
			want: WeightedScore{Score: 0, FilesTouched: 0, ScatterPenalty: 0},
		},
		{
			name: "binary files with no additions don't count",
			stats: &diff.DiffStats{
				Files: []diff.FileStat{
					{Path: "image.png", IsBinary: true, Additions: 0},
				},
				TotalFiles: 1,
			},
			want: WeightedScore{Score: 0, FilesTouched: 0},
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

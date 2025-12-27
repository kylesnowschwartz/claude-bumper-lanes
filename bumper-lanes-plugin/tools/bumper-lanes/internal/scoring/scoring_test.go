package scoring

import (
	"testing"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name      string
		stats     *diff.StatsJSON
		wantScore int
		wantNew   int
		wantEdit  int
	}{
		{
			name: "empty stats",
			stats: &diff.StatsJSON{
				Files:  []diff.FileStatJSON{},
				Totals: diff.TotalsJSON{Adds: 0, Dels: 0, FileCount: 0},
			},
			wantScore: 0,
		},
		{
			name: "new file only - 1.0x weight",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "new.go", Adds: 100, New: true},
				},
				Totals: diff.TotalsJSON{Adds: 100, FileCount: 1},
			},
			wantScore: 100, // 100 * 1.0 = 100
			wantNew:   100,
			wantEdit:  0,
		},
		{
			name: "edit only - 1.3x weight",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "existing.go", Adds: 100, New: false},
				},
				Totals: diff.TotalsJSON{Adds: 100, FileCount: 1},
			},
			wantScore: 130, // 100 * 1.3 = 130
			wantNew:   0,
			wantEdit:  100,
		},
		{
			name: "mixed new and edit",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "new.go", Adds: 50, New: true},
					{Path: "edit.go", Adds: 50, New: false},
				},
				Totals: diff.TotalsJSON{Adds: 100, FileCount: 2},
			},
			wantScore: 115, // (50*1.0) + (50*1.3) = 50 + 65 = 115
			wantNew:   50,
			wantEdit:  50,
		},
		{
			name: "scatter penalty - 6 files",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "a.go", Adds: 10}, {Path: "b.go", Adds: 10},
					{Path: "c.go", Adds: 10}, {Path: "d.go", Adds: 10},
					{Path: "e.go", Adds: 10}, {Path: "f.go", Adds: 10},
				},
				Totals: diff.TotalsJSON{Adds: 60, FileCount: 6},
			},
			wantScore: 88, // 60*1.3=78 + (6-5)*10=10 = 88
		},
		{
			name: "scatter penalty - 11 files high tier",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "a.go", Adds: 5}, {Path: "b.go", Adds: 5},
					{Path: "c.go", Adds: 5}, {Path: "d.go", Adds: 5},
					{Path: "e.go", Adds: 5}, {Path: "f.go", Adds: 5},
					{Path: "g.go", Adds: 5}, {Path: "h.go", Adds: 5},
					{Path: "i.go", Adds: 5}, {Path: "j.go", Adds: 5},
					{Path: "k.go", Adds: 5},
				},
				Totals: diff.TotalsJSON{Adds: 55, FileCount: 11},
			},
			wantScore: 251, // 55*1.3=71 + (11-5)*30=180 = 251
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calculate(tt.stats)
			if got.Score != tt.wantScore {
				t.Errorf("Score = %d, want %d", got.Score, tt.wantScore)
			}
			if tt.wantNew > 0 && got.NewAdditions != tt.wantNew {
				t.Errorf("NewAdditions = %d, want %d", got.NewAdditions, tt.wantNew)
			}
			if tt.wantEdit > 0 && got.EditAdditions != tt.wantEdit {
				t.Errorf("EditAdditions = %d, want %d", got.EditAdditions, tt.wantEdit)
			}
		})
	}
}

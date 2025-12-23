// Package scoring implements bumper-lanes weighted threshold scoring.
// This calculates scores from raw DiffStats JSON (from git-diff-tree --stats-json).
package scoring

// StatsJSON matches the output of git-diff-tree --stats-json.
// This is duplicated to avoid importing from diff-viz.
type StatsJSON struct {
	Files  []FileStatJSON `json:"files"`
	Totals TotalsJSON     `json:"totals"`
}

// FileStatJSON represents a single file's stats.
type FileStatJSON struct {
	Path   string `json:"path"`
	Adds   int    `json:"adds"`
	Dels   int    `json:"dels"`
	Binary bool   `json:"binary,omitempty"`
	New    bool   `json:"new,omitempty"`
}

// TotalsJSON represents aggregate stats.
type TotalsJSON struct {
	Adds      int `json:"adds"`
	Dels      int `json:"dels"`
	FileCount int `json:"fileCount"`
}

// WeightedScore holds the bumper-lanes weighted score calculation.
type WeightedScore struct {
	Score          int `json:"score"`          // Total weighted score
	NewAdditions   int `json:"new_additions"`  // Lines added in new files
	EditAdditions  int `json:"edit_additions"` // Lines added in edited files
	FilesTouched   int `json:"files_touched"`  // Number of files changed
	ScatterPenalty int `json:"scatter"`        // Penalty for touching many files
}

// Scoring constants (match threshold-calculator.sh)
const (
	newFileWeight        = 10 // 1.0x baseline (scaled x10 for integer math)
	editFileWeight       = 13 // 1.3x penalty (edits harder to review)
	scatterLowThreshold  = 6  // Medium penalty starts
	scatterHighThreshold = 11 // High penalty starts
	scatterPenaltyLow    = 10 // Points/file for 6-10 files
	scatterPenaltyHigh   = 30 // Points/file for 11+ files
	freeTier             = 5  // Files 1-5 are penalty-free
)

// Calculate computes bumper-lanes score from raw diff stats.
// New files get 1.0x weight, edits get 1.3x weight.
func Calculate(stats *StatsJSON) *WeightedScore {
	var newAdd, editAdd int

	for _, f := range stats.Files {
		if f.New {
			newAdd += f.Adds
		} else {
			editAdd += f.Adds
		}
	}

	// Calculate scatter penalty
	var scatter int
	fileCount := stats.Totals.FileCount
	if fileCount >= scatterHighThreshold {
		scatter = (fileCount - freeTier) * scatterPenaltyHigh
	} else if fileCount >= scatterLowThreshold {
		scatter = (fileCount - freeTier) * scatterPenaltyLow
	}

	// Weighted score: (new x 10 + edit x 13) / 10 + scatter
	totalPoints := (newAdd * newFileWeight) + (editAdd * editFileWeight)
	score := (totalPoints / 10) + scatter

	return &WeightedScore{
		Score:          score,
		NewAdditions:   newAdd,
		EditAdditions:  editAdd,
		FilesTouched:   fileCount,
		ScatterPenalty: scatter,
	}
}

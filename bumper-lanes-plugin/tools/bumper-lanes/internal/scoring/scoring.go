// Package scoring implements bumper-lanes weighted threshold scoring.
// This calculates scores from raw DiffStats JSON (from diff-viz library).
package scoring

import "github.com/kylesnowschwartz/diff-viz/v2/diff"

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
// Deletions are ignored (they reduce complexity, not add review burden).
func Calculate(stats *diff.StatsJSON) *WeightedScore {
	var newAdd, editAdd int
	var filesWithAdditions int // Only count files that add lines (not pure deletions)

	for _, f := range stats.Files {
		if f.Adds > 0 {
			filesWithAdditions++
			if f.New {
				newAdd += f.Adds
			} else {
				editAdd += f.Adds
			}
		}
		// Files with only deletions (f.Adds == 0) don't count toward scatter
	}

	// Calculate scatter penalty (only for files with additions)
	var scatter int
	if filesWithAdditions >= scatterHighThreshold {
		scatter = (filesWithAdditions - freeTier) * scatterPenaltyHigh
	} else if filesWithAdditions >= scatterLowThreshold {
		scatter = (filesWithAdditions - freeTier) * scatterPenaltyLow
	}

	// Weighted score: (new x 10 + edit x 13) / 10 + scatter
	totalPoints := (newAdd * newFileWeight) + (editAdd * editFileWeight)
	score := (totalPoints / 10) + scatter

	return &WeightedScore{
		Score:          score,
		NewAdditions:   newAdd,
		EditAdditions:  editAdd,
		FilesTouched:   filesWithAdditions, // Only files with additions
		ScatterPenalty: scatter,
	}
}

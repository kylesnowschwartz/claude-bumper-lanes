package scoring

import (
	"testing"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
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

// TestCalculateGeneratedFilesScoreZero verifies lockfile/codegen/vendored
// churn is free: it is machine-written and nobody reviews it line by line.
func TestCalculateGeneratedFilesScoreZero(t *testing.T) {
	stats := &diff.StatsJSON{
		Files: []diff.FileStatJSON{
			{Path: "go.sum", Adds: 500},
			{Path: "package-lock.json", Adds: 2000},
			{Path: "vendor/dep/lib.go", Adds: 900},
			{Path: "api/gen_generated.go", Adds: 300},
			{Path: "assets/app.min.js", Adds: 5000},
			{Path: "main.go", Adds: 10},
		},
		Totals: diff.TotalsJSON{Adds: 8710, FileCount: 6},
	}

	got := Calculate(stats)
	if got.Score != 13 { // only main.go: 10 * 1.3; generated files excluded from score AND scatter
		t.Errorf("Score = %d, want 13 (generated churn must be free)", got.Score)
	}
	if got.FilesTouched != 1 {
		t.Errorf("FilesTouched = %d, want 1 (generated files must not count toward scatter)", got.FilesTouched)
	}
}

// TestByModuleTiebreakOrdering verifies modules with equal points are
// ordered alphabetically by module path, per the documented tiebreak.
func TestByModuleTiebreakOrdering(t *testing.T) {
	stats := &diff.StatsJSON{
		Files: []diff.FileStatJSON{
			{Path: "dirB/file.go", Adds: 10, New: false},
			{Path: "dirA/file.go", Adds: 10, New: false},
		},
	}

	got := ByModule(stats)
	if len(got) != 2 {
		t.Fatalf("len(ByModule) = %d, want 2", len(got))
	}
	if got[0].Points != got[1].Points {
		t.Fatalf("expected equal points to exercise tiebreak, got %d and %d", got[0].Points, got[1].Points)
	}
	if got[0].Module != "dirA/" || got[1].Module != "dirB/" {
		t.Errorf("Modules = [%q, %q], want [\"dirA/\", \"dirB/\"] (alphabetical tiebreak)", got[0].Module, got[1].Module)
	}
}

// TestByModuleRootLevelAttribution verifies a file with no directory prefix
// is attributed to the "(root)" module.
func TestByModuleRootLevelAttribution(t *testing.T) {
	stats := &diff.StatsJSON{
		Files: []diff.FileStatJSON{
			{Path: "main.go", Adds: 10, New: true},
		},
	}

	got := ByModule(stats)
	if len(got) != 1 {
		t.Fatalf("len(ByModule) = %d, want 1", len(got))
	}
	if got[0].Module != "(root)" {
		t.Errorf("Module = %q, want \"(root)\"", got[0].Module)
	}
	if got[0].Points != 10 || got[0].Files != 1 {
		t.Errorf("Points/Files = %d/%d, want 10/1", got[0].Points, got[0].Files)
	}
}

// TestByModuleExcludesGeneratedFiles verifies generated files contribute no
// points to any module and do not appear in the output at all.
func TestByModuleExcludesGeneratedFiles(t *testing.T) {
	stats := &diff.StatsJSON{
		Files: []diff.FileStatJSON{
			{Path: "go.sum", Adds: 500, New: false},
			{Path: "vendor/dep/lib.go", Adds: 900, New: true},
		},
	}

	got := ByModule(stats)
	if len(got) != 0 {
		t.Errorf("ByModule(generated-only) = %+v, want empty (generated files excluded entirely)", got)
	}
}

// TestByModuleSkipsZeroAddFiles verifies files with Adds==0 (pure deletions)
// are skipped, not attributed to their module with zero points.
func TestByModuleSkipsZeroAddFiles(t *testing.T) {
	stats := &diff.StatsJSON{
		Files: []diff.FileStatJSON{
			{Path: "dirA/removed.go", Adds: 0, Dels: 40, New: false},
			{Path: "dirA/kept.go", Adds: 10, New: false},
		},
	}

	got := ByModule(stats)
	if len(got) != 1 {
		t.Fatalf("len(ByModule) = %d, want 1 (zero-add file must be skipped)", len(got))
	}
	if got[0].Files != 1 {
		t.Errorf("Files = %d, want 1 (only the file with additions counts)", got[0].Files)
	}
}

// TestByModuleCalculateInvariant pins the documented invariant (scoring.go
// ~71-76): module points sum to Calculate's score minus its scatter penalty.
// Adds values are multiples of 10 throughout so per-file truncation in
// ByModule and aggregate truncation in Calculate coincide exactly.
func TestByModuleCalculateInvariant(t *testing.T) {
	tests := []struct {
		name  string
		stats *diff.StatsJSON
	}{
		{
			name:  "empty",
			stats: &diff.StatsJSON{Files: []diff.FileStatJSON{}},
		},
		{
			name: "single new file",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "pkg/new.go", Adds: 20, New: true},
				},
			},
		},
		{
			name: "mixed new and edit across directories",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "dirA/new.go", Adds: 20, New: true},
					{Path: "dirB/edit.go", Adds: 30, New: false},
					{Path: "main.go", Adds: 10, New: true},
				},
			},
		},
		{
			name: "with generated files and zero-add files mixed in",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "dirA/new.go", Adds: 20, New: true},
					{Path: "dirB/edit.go", Adds: 30, New: false},
					{Path: "go.sum", Adds: 500, New: false},
					{Path: "dirC/removed.go", Adds: 0, Dels: 40, New: false},
				},
			},
		},
		{
			name: "scatter-triggering fan-out",
			stats: &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "d1/a.go", Adds: 10}, {Path: "d2/b.go", Adds: 10},
					{Path: "d3/c.go", Adds: 10}, {Path: "d4/d.go", Adds: 10},
					{Path: "d5/e.go", Adds: 10}, {Path: "d6/f.go", Adds: 10},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := Calculate(tt.stats)
			modules := ByModule(tt.stats)

			var sum int
			for _, m := range modules {
				sum += m.Points
			}

			want := calc.Score - calc.ScatterPenalty
			if sum != want {
				t.Errorf("sum(ByModule points) = %d, want %d (Score %d - ScatterPenalty %d)",
					sum, want, calc.Score, calc.ScatterPenalty)
			}
		})
	}
}

// TestCalculateIntegerTruncationBoundaries pins Calculate's integer
// truncation ((edit*13)/10) at values that don't divide evenly, so a future
// change to the arithmetic must consciously touch this test.
func TestCalculateIntegerTruncationBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		adds      int
		wantScore int
	}{
		{name: "adds=1", adds: 1, wantScore: 1},    // 1*13=13, 13/10=1
		{name: "adds=3", adds: 3, wantScore: 3},    // 3*13=39, 39/10=3
		{name: "adds=7", adds: 7, wantScore: 9},    // 7*13=91, 91/10=9
		{name: "adds=10", adds: 10, wantScore: 13}, // 10*13=130, 130/10=13 (evenly divisible)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &diff.StatsJSON{
				Files: []diff.FileStatJSON{
					{Path: "edit.go", Adds: tt.adds, New: false},
				},
			}
			got := Calculate(stats)
			if got.Score != tt.wantScore {
				t.Errorf("Calculate(Adds=%d edit).Score = %d, want %d", tt.adds, got.Score, tt.wantScore)
			}
		})
	}
}

func TestIsGenerated(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"go.sum", true},
		{"tools/api/go.sum", true},
		{"Cargo.lock", true},
		{"flake.lock", true}, // .lock suffix
		{"vendor/x/y.go", true},
		{"app/node_modules/pkg/index.js", true},
		{"api/service.pb.go", true},
		{"models_generated.go", true},
		{"dist/app.min.css", true},
		{"go.mod", false},
		{"main.go", false},
		{"locksmith.go", false},
		{"vendors.go", false},
	}
	for _, tt := range tests {
		if got := IsGenerated(tt.path); got != tt.want {
			t.Errorf("IsGenerated(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

package statusline

import (
	"os"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/testutil"
)

func TestRenderBumperWidget(t *testing.T) {
	t.Run("no session file returns empty output", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.SetupTempGitRepo(t, tmpDir)
		t.Chdir(tmpDir)

		out := renderBumperWidget("no-such-session")
		if out.State != "" {
			t.Errorf("State = %q, want empty", out.State)
		}
		if out.BumperIndicator != "" {
			t.Errorf("BumperIndicator = %q, want empty", out.BumperIndicator)
		}
	})

	t.Run("threshold limit zero means disabled", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.SetupTempGitRepo(t, tmpDir)
		t.Chdir(tmpDir)

		sess, err := state.New("sess-disabled", "tree1", "main", 0)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		if err := sess.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		out := renderBumperWidget("sess-disabled")
		if out.State != "disabled" {
			t.Errorf("State = %q, want %q", out.State, "disabled")
		}
	})

	t.Run("paused session", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.SetupTempGitRepo(t, tmpDir)
		t.Chdir(tmpDir)

		sess, err := state.New("sess-paused", "tree1", "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.SetPaused(true)
		if err := sess.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		out := renderBumperWidget("sess-paused")
		if out.State != "paused" {
			t.Errorf("State = %q, want %q", out.State, "paused")
		}
	})

	t.Run("stop triggered means tripped", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.SetupTempGitRepo(t, tmpDir)
		t.Chdir(tmpDir)

		sess, err := state.New("sess-tripped", "tree1", "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.SetStopTriggered(true)
		sess.SetScore(650)
		if err := sess.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		out := renderBumperWidget("sess-tripped")
		if out.State != "tripped" {
			t.Errorf("State = %q, want %q", out.State, "tripped")
		}
	})

	t.Run("active state computes percentage", func(t *testing.T) {
		tests := []struct {
			name    string
			score   int
			limit   int
			wantPct int
		}{
			{"25 percent", 150, 600, 25},
			{"50 percent", 300, 600, 50},
			{"over 100 percent", 700, 600, 116},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tmpDir := t.TempDir()
				testutil.SetupTempGitRepo(t, tmpDir)
				t.Chdir(tmpDir)

				sess, err := state.New("sess-active", "tree1", "main", tt.limit)
				if err != nil {
					t.Fatalf("state.New() error = %v", err)
				}
				sess.SetScore(tt.score)
				if err := sess.Save(); err != nil {
					t.Fatalf("Save() error = %v", err)
				}

				out := renderBumperWidget("sess-active")
				if out.State != "active" {
					t.Errorf("State = %q, want %q", out.State, "active")
				}
				if out.Percentage != tt.wantPct {
					t.Errorf("Percentage = %d, want %d", out.Percentage, tt.wantPct)
				}
				if out.BumperIndicator == "" {
					t.Errorf("BumperIndicator is empty, want non-empty")
				}
			})
		}
	})

	t.Run("tripwires present", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.SetupTempGitRepo(t, tmpDir)
		t.Chdir(tmpDir)

		sess, err := state.New("sess-tripwire", "tree1", "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.AddTripwires([]string{"go.mod"})
		if err := sess.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		out := renderBumperWidget("sess-tripwire")
		if out.State != "active" {
			t.Errorf("State = %q, want %q", out.State, "active")
		}
		if !strings.Contains(out.BumperIndicator, "⚠") {
			t.Errorf("BumperIndicator = %q, want it to contain the tripwire glyph %q", out.BumperIndicator, "⚠")
		}
	})
}

func TestRenderIndicator(t *testing.T) {
	tmpDir := t.TempDir()
	testutil.SetupTempGitRepo(t, tmpDir)

	sess, err := state.New("sess-indicator", "tree1", "main", 600)
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}
	sess.SetScore(300) // 300/600 = 50%

	t.Run("save session", func(t *testing.T) {
		t.Chdir(tmpDir)
		if err := sess.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	})

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	input := &StatusInput{
		SessionID: "sess-indicator",
		Workspace: struct {
			CurrentDir string `json:"current_dir"`
		}{CurrentDir: tmpDir},
	}

	got, err := RenderIndicator(input)
	if err != nil {
		t.Fatalf("RenderIndicator() error = %v", err)
	}

	if got.State != "active" {
		t.Errorf("RenderIndicator().State = %q, want %q", got.State, "active")
	}
	if !strings.Contains(got.BumperIndicator, "50%") {
		t.Errorf("RenderIndicator().BumperIndicator = %q, want it to contain %q", got.BumperIndicator, "50%")
	}

	newDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if newDir != origDir {
		t.Errorf("cwd after RenderIndicator() = %q, want %q (not restored)", newDir, origDir)
	}
}

func TestRender(t *testing.T) {
	t.Run("with saved session includes bumper segment", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.SetupTempGitRepo(t, tmpDir)

		sess, err := state.New("sess-render", "tree1", "main", 600)
		if err != nil {
			t.Fatalf("state.New() error = %v", err)
		}
		sess.SetScore(300)
		t.Run("save session", func(t *testing.T) {
			t.Chdir(tmpDir)
			if err := sess.Save(); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
		})

		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd() error = %v", err)
		}

		input := &StatusInput{
			SessionID: "sess-render",
			Workspace: struct {
				CurrentDir string `json:"current_dir"`
			}{CurrentDir: tmpDir},
		}
		input.Model.DisplayName = "Sonnet"
		input.Cost.TotalCostUSD = 1.23

		out, err := Render(input)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		if !strings.Contains(out.StatusLine, "Sonnet") {
			t.Errorf("StatusLine missing model name: %s", out.StatusLine)
		}
		if !strings.Contains(out.StatusLine, "main") {
			t.Errorf("StatusLine missing branch name: %s", out.StatusLine)
		}
		if !strings.Contains(out.StatusLine, "$1.23") {
			t.Errorf("StatusLine missing cost: %s", out.StatusLine)
		}
		if out.BumperIndicator == "" || !strings.Contains(out.StatusLine, out.BumperIndicator) {
			t.Errorf("StatusLine missing bumper indicator: %s", out.StatusLine)
		}

		newDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd() error = %v", err)
		}
		if newDir != origDir {
			t.Errorf("cwd after Render() = %q, want %q (not restored)", newDir, origDir)
		}
	})

	t.Run("no saved session omits bumper segment", func(t *testing.T) {
		tmpDir := t.TempDir()
		testutil.SetupTempGitRepo(t, tmpDir)

		input := &StatusInput{
			SessionID: "sess-no-state",
			Workspace: struct {
				CurrentDir string `json:"current_dir"`
			}{CurrentDir: tmpDir},
		}
		input.Model.DisplayName = "Sonnet"
		input.Cost.TotalCostUSD = 0.42

		out, err := Render(input)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		if !strings.Contains(out.StatusLine, "Sonnet") {
			t.Errorf("StatusLine missing model name: %s", out.StatusLine)
		}
		if !strings.Contains(out.StatusLine, "$0.42") {
			t.Errorf("StatusLine missing cost: %s", out.StatusLine)
		}
		if out.BumperIndicator != "" {
			t.Errorf("BumperIndicator = %q, want empty (no session)", out.BumperIndicator)
		}
	})
}

func TestParseInput(t *testing.T) {
	t.Run("parses complete input", func(t *testing.T) {
		input := `{
			"session_id": "abc123",
			"model": {"display_name": "Sonnet"},
			"workspace": {"current_dir": "/home/user/project"},
			"cost": {"total_cost_usd": 1.23}
		}`

		got, err := ParseInput([]byte(input))
		if err != nil {
			t.Fatalf("ParseInput() error = %v", err)
		}

		if got.SessionID != "abc123" {
			t.Errorf("SessionID = %q, want %q", got.SessionID, "abc123")
		}
		if got.Model.DisplayName != "Sonnet" {
			t.Errorf("Model.DisplayName = %q, want %q", got.Model.DisplayName, "Sonnet")
		}
		if got.Workspace.CurrentDir != "/home/user/project" {
			t.Errorf("Workspace.CurrentDir = %q, want %q", got.Workspace.CurrentDir, "/home/user/project")
		}
		if got.Cost.TotalCostUSD != 1.23 {
			t.Errorf("Cost.TotalCostUSD = %f, want %f", got.Cost.TotalCostUSD, 1.23)
		}
	})

	t.Run("handles minimal input", func(t *testing.T) {
		input := `{"session_id": "sess-001"}`

		got, err := ParseInput([]byte(input))
		if err != nil {
			t.Fatalf("ParseInput() error = %v", err)
		}

		if got.SessionID != "sess-001" {
			t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-001")
		}
		// Zero values for optional fields
		if got.Model.DisplayName != "" {
			t.Errorf("Model.DisplayName = %q, want empty", got.Model.DisplayName)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		input := `{not valid json}`

		_, err := ParseInput([]byte(input))
		if err == nil {
			t.Error("ParseInput() should error on invalid JSON")
		}
	})
}

func TestFormatBumperStatus(t *testing.T) {
	tests := []struct {
		name         string
		state        string
		percentage   int
		hasTripwires bool
		netLines     int
		wantColor    string
		wantBar      bool // true if expecting traffic light bar
		wantText     string
	}{
		{
			name:       "active state <70% shows green bar",
			state:      "active",
			percentage: 25,
			wantColor:  colorGreen,
			wantBar:    true,
		},
		{
			name:       "active state 70-90% shows yellow bar",
			state:      "active",
			percentage: 80,
			wantColor:  colorYellow,
			wantBar:    true,
		},
		{
			name:       "active state >90% shows red bar",
			state:      "active",
			percentage: 95,
			wantColor:  colorRed,
			wantBar:    true,
		},
		{
			name:       "tripped state shows red bar",
			state:      "tripped",
			percentage: 112,
			wantColor:  colorRed,
			wantBar:    true,
		},
		{
			name:      "paused state shows yellow text",
			state:     "paused",
			wantColor: colorYellow,
			wantText:  "Paused",
		},
		{
			name:      "disabled state shows blue text",
			state:     "disabled",
			wantColor: colorBlue,
			wantText:  "Disabled",
		},
		{
			name:         "tripwire hit shows warning glyph",
			state:        "active",
			percentage:   25,
			hasTripwires: true,
			wantColor:    colorRed,
			wantBar:      true,
			wantText:     "⚠",
		},
		{
			name:       "net-negative increment shows green line count",
			state:      "active",
			percentage: 10,
			netLines:   -42,
			wantColor:  colorGreen,
			wantBar:    true,
			wantText:   "-42 lines",
		},
		{
			name:       "net-positive increment shows no line count",
			state:      "active",
			percentage: 10,
			netLines:   42,
			wantColor:  colorGreen,
			wantBar:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBumperStatus(tt.state, tt.percentage, tt.hasTripwires, tt.netLines)

			if !strings.Contains(got, tt.wantColor) {
				t.Errorf("formatBumperStatus() missing color %q in: %s", tt.wantColor, got)
			}
			if tt.wantBar {
				// Traffic light uses ▂ (short/green), ▄ (medium/yellow), █ (tall/red)
				hasBar := strings.Contains(got, "▂") || strings.Contains(got, "▄") || strings.Contains(got, "█")
				if !hasBar {
					t.Errorf("formatBumperStatus() missing bar chars in: %s", got)
				}
				// Should include percentage
				if !strings.Contains(got, "%") {
					t.Errorf("formatBumperStatus() missing percentage in: %s", got)
				}
			}
			if tt.wantText != "" && !strings.Contains(got, tt.wantText) {
				t.Errorf("formatBumperStatus() missing text %q in: %s", tt.wantText, got)
			}
			if tt.netLines >= 0 && strings.Contains(got, "lines") {
				t.Errorf("formatBumperStatus() should not show line count for net-positive: %s", got)
			}
		})
	}
}

func TestFormatOutput(t *testing.T) {
	t.Run("widget=all formats full output", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine:      "[Sonnet] | project | main | $1.23",
			BumperIndicator: "▂ 25%",
		}

		got := FormatOutput(out, WidgetAll)
		if !strings.Contains(got, out.StatusLine) {
			t.Errorf("FormatOutput(all) missing status line")
		}
	})

	t.Run("widget=indicator returns only bumper indicator", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine:      "[Sonnet] | project | main | $1.23 | ▂ 25%",
			BumperIndicator: "▂ 25%",
		}

		got := FormatOutput(out, WidgetIndicator)

		// Should have indicator with bar chars (▂, ▄, or █)
		hasBar := strings.Contains(got, "▂") || strings.Contains(got, "▄") || strings.Contains(got, "█")
		if !hasBar {
			t.Errorf("FormatOutput(indicator) missing bar char, got: %q", got)
		}
		// Should NOT have full status line parts
		if strings.Contains(got, "Sonnet") {
			t.Errorf("FormatOutput(indicator) should not include model name")
		}
	})

	t.Run("widget=diff-tree from pre-v4 wrappers returns nothing", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine:      "[Sonnet] | project",
			BumperIndicator: "▂ 25%",
		}

		got := FormatOutput(out, "diff-tree")
		if got != "" {
			t.Errorf("FormatOutput(diff-tree) = %q, want empty (removed widget)", got)
		}
	})

	t.Run("handles empty status line", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine: "",
		}

		got := FormatOutput(out, WidgetAll)
		if got != "" {
			t.Errorf("FormatOutput() with empty status = %q, want empty", got)
		}
	})

	t.Run("handles empty indicator", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine:      "[Sonnet] | project",
			BumperIndicator: "",
		}

		got := FormatOutput(out, WidgetIndicator)
		if got != "" {
			t.Errorf("FormatOutput(indicator) with empty indicator = %q, want empty", got)
		}
	})

	t.Run("default widget is all", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine: "[Sonnet] | project",
		}

		// Empty string should behave like "all"
		got := FormatOutput(out, "")
		if !strings.Contains(got, out.StatusLine) {
			t.Errorf("FormatOutput('') should default to all, got: %q", got)
		}
	})
}

package statusline

import (
	"strings"
	"testing"
)

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
		name       string
		state      string
		score      int
		limit      int
		percentage int
		wantColor  string
		wantText   string
	}{
		{
			name:       "active state shows green",
			state:      "active",
			score:      100,
			limit:      400,
			percentage: 25,
			wantColor:  colorGreen,
			wantText:   "active (100/400 - 25%)",
		},
		{
			name:       "tripped state shows red",
			state:      "tripped",
			score:      450,
			limit:      400,
			percentage: 112,
			wantColor:  colorRed,
			wantText:   "tripped (450/400 - 112%)",
		},
		{
			name:       "paused state shows yellow with command hint",
			state:      "paused",
			score:      0, // score/limit ignored for paused
			limit:      0,
			percentage: 0,
			wantColor:  colorYellow,
			wantText:   "Paused: /bumper-resume",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBumperStatus(tt.state, tt.score, tt.limit, tt.percentage)

			if !strings.Contains(got, tt.wantColor) {
				t.Errorf("formatBumperStatus() missing color %q in: %s", tt.wantColor, got)
			}
			if !strings.Contains(got, tt.wantText) {
				t.Errorf("formatBumperStatus() missing text %q in: %s", tt.wantText, got)
			}
			if !strings.HasSuffix(got, colorReset) {
				t.Errorf("formatBumperStatus() should end with color reset")
			}
		})
	}
}

func TestFormatOutput(t *testing.T) {
	t.Run("formats status line only", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine: "[Sonnet] | project | main | $1.23",
		}

		got := FormatOutput(out)
		if !strings.Contains(got, out.StatusLine) {
			t.Errorf("FormatOutput() missing status line")
		}
		if !strings.HasSuffix(got, "\n") {
			t.Errorf("FormatOutput() should end with newline")
		}
	})

	t.Run("formats with diff tree", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine: "[Sonnet] | project",
			DiffTree:   "├── src\n│   └── main.go +10",
		}

		got := FormatOutput(out)

		// Should contain both parts
		if !strings.Contains(got, out.StatusLine) {
			t.Errorf("FormatOutput() missing status line")
		}

		// Diff tree lines should have ANSI reset prefix
		if !strings.Contains(got, "\033[0m") {
			t.Errorf("FormatOutput() missing ANSI reset in diff tree")
		}

		// Spaces should be converted to non-breaking spaces
		if strings.Contains(got, "│   └") {
			t.Errorf("FormatOutput() should convert spaces to non-breaking spaces")
		}
		// Non-breaking space version
		if !strings.Contains(got, "│\u00A0\u00A0\u00A0└") {
			t.Errorf("FormatOutput() should use non-breaking spaces, got: %q", got)
		}
	})

	t.Run("handles empty status line", func(t *testing.T) {
		out := &StatusOutput{
			StatusLine: "",
		}

		got := FormatOutput(out)
		if got != "" {
			t.Errorf("FormatOutput() with empty status = %q, want empty", got)
		}
	})
}


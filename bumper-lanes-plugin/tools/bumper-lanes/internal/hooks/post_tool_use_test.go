package hooks

import (
	"testing"
)

func TestFuelGaugeTier(t *testing.T) {
	threshold := 400

	tests := []struct {
		name      string
		score     int
		wantTier  string
		wantQuiet bool
	}{
		{
			name:      "0% - silent",
			score:     0,
			wantTier:  "",
			wantQuiet: true,
		},
		{
			name:      "25% - silent",
			score:     100,
			wantTier:  "",
			wantQuiet: true,
		},
		{
			name:      "49% - silent",
			score:     196,
			wantTier:  "",
			wantQuiet: true,
		},
		{
			name:      "50% - notice",
			score:     200,
			wantTier:  "NOTICE",
			wantQuiet: false,
		},
		{
			name:      "60% - notice",
			score:     240,
			wantTier:  "NOTICE",
			wantQuiet: false,
		},
		{
			name:      "74% - notice",
			score:     296,
			wantTier:  "NOTICE",
			wantQuiet: false,
		},
		{
			name:      "75% - warning",
			score:     300,
			wantTier:  "WARNING",
			wantQuiet: false,
		},
		{
			name:      "85% - warning",
			score:     340,
			wantTier:  "WARNING",
			wantQuiet: false,
		},
		{
			name:      "89% - warning",
			score:     356,
			wantTier:  "WARNING",
			wantQuiet: false,
		},
		{
			name:      "90% - critical",
			score:     360,
			wantTier:  "CRITICAL",
			wantQuiet: false,
		},
		{
			name:      "100% - critical",
			score:     400,
			wantTier:  "CRITICAL",
			wantQuiet: false,
		},
		{
			name:      "150% - critical",
			score:     600,
			wantTier:  "CRITICAL",
			wantQuiet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, quiet := getFuelGaugeTier(tt.score, threshold)
			if tier != tt.wantTier {
				t.Errorf("getFuelGaugeTier(%d, %d) tier = %q, want %q", tt.score, threshold, tier, tt.wantTier)
			}
			if quiet != tt.wantQuiet {
				t.Errorf("getFuelGaugeTier(%d, %d) quiet = %v, want %v", tt.score, threshold, quiet, tt.wantQuiet)
			}
		})
	}
}

func TestFuelGaugeMessage(t *testing.T) {
	tests := []struct {
		tier        string
		score       int
		threshold   int
		wantContain string
	}{
		{"NOTICE", 220, 400, "55%"},
		{"WARNING", 320, 400, "80%"},
		{"CRITICAL", 380, 400, "95%"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			msg := formatFuelGaugeMessage(tt.tier, tt.score, tt.threshold)
			if !containsSubstring(msg, tt.tier) {
				t.Errorf("message should contain tier %q, got: %s", tt.tier, msg)
			}
			if !containsSubstring(msg, tt.wantContain) {
				t.Errorf("message should contain %q, got: %s", tt.wantContain, msg)
			}
		})
	}
}

// getFuelGaugeTier calculates the warning tier based on score vs threshold
func getFuelGaugeTier(score, threshold int) (tier string, quiet bool) {
	if threshold <= 0 {
		return "", true
	}

	percent := (score * 100) / threshold

	switch {
	case percent >= 90:
		return "CRITICAL", false
	case percent >= 75:
		return "WARNING", false
	case percent >= 50:
		return "NOTICE", false
	default:
		return "", true
	}
}

// formatFuelGaugeMessage creates the warning message
func formatFuelGaugeMessage(tier string, score, threshold int) string {
	percent := (score * 100) / threshold
	return tier + ": Review budget at " + itoa(percent) + "%. " + itoa(score) + "/" + itoa(threshold) + " pts."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

package statusline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsOurBinary(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"empty cmd", "", false},
		{"our binary", "/some/path/bumper-lanes", true},
		{"foreign script", "/usr/bin/my-status.sh", false},
		{"foreign binary sharing a prefix", "/usr/bin/bumper-lanes-extra", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOurBinary(tt.cmd); got != tt.want {
				t.Errorf("isOurBinary(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestGetStatusLineCommand(t *testing.T) {
	homeDir := t.TempDir()
	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")

	t.Run("returns command when configured", func(t *testing.T) {
		os.WriteFile(settingsPath, []byte(`{"statusLine": {"type": "command", "command": "/usr/bin/my-status"}}`), 0644)
		if got := getStatusLineCommand(homeDir); got != "/usr/bin/my-status" {
			t.Errorf("getStatusLineCommand() = %q, want /usr/bin/my-status", got)
		}
	})

	t.Run("returns empty when statusLine absent", func(t *testing.T) {
		os.WriteFile(settingsPath, []byte(`{"theme": "dark"}`), 0644)
		if got := getStatusLineCommand(homeDir); got != "" {
			t.Errorf("getStatusLineCommand() = %q, want empty", got)
		}
	})

	t.Run("returns empty when settings file missing", func(t *testing.T) {
		os.Remove(settingsPath)
		if got := getStatusLineCommand(homeDir); got != "" {
			t.Errorf("getStatusLineCommand() = %q, want empty", got)
		}
	})
}

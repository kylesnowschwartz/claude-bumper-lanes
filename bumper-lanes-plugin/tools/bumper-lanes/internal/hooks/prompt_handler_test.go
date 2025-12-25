package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSetupStatuslineCommandPattern verifies the command is recognized.
func TestSetupStatuslineCommandPattern(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		matches bool
	}{
		{"short form", "/bumper-setup-statusline", true},
		{"full form", "/claude-bumper-lanes:bumper-setup-statusline", true},
		{"with trailing space", "/bumper-setup-statusline ", true},
		{"wrong command", "/bumper-setup", false},
		{"partial match", "/bumper-setup-statuslin", false},
		{"extra suffix", "/bumper-setup-statusline-foo", false},
		{"with argument", "/bumper-setup-statusline arg", false}, // no args allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setupStatuslineCmdPattern.MatchString(tt.prompt)
			if got != tt.matches {
				t.Errorf("setupStatuslineCmdPattern.MatchString(%q) = %v, want %v",
					tt.prompt, got, tt.matches)
			}
		})
	}
}

// TestGetBumperLanesBinPath verifies path detection works.
func TestGetBumperLanesBinPath(t *testing.T) {
	path := getBumperLanesBinPath()

	// Should return non-empty string
	if path == "" {
		t.Error("getBumperLanesBinPath() returned empty string")
	}

	// Should be an absolute path or "bumper-lanes" fallback
	if path != "bumper-lanes" && !filepath.IsAbs(path) {
		t.Errorf("getBumperLanesBinPath() = %q, want absolute path or 'bumper-lanes'", path)
	}
}

// TestGetAddonScriptPath verifies addon script path detection.
func TestGetAddonScriptPath(t *testing.T) {
	path := getAddonScriptPath()

	// Should return non-empty string
	if path == "" {
		t.Error("getAddonScriptPath() returned empty string")
	}

	// Should end with the expected filename
	if filepath.Base(path) != "bumper-lanes-addon.sh" {
		t.Errorf("getAddonScriptPath() = %q, want path ending with 'bumper-lanes-addon.sh'", path)
	}

	// Should contain status-lines directory
	if filepath.Base(filepath.Dir(path)) != "status-lines" {
		t.Errorf("getAddonScriptPath() = %q, want path containing 'status-lines' directory", path)
	}
}

// TestHasStatusLineConfigured tests status line detection.
// Uses temp HOME to avoid affecting real user settings.
func TestHasStatusLineConfigured(t *testing.T) {
	// Save and restore HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	t.Run("returns false when no settings file", func(t *testing.T) {
		tmpHome := t.TempDir()
		os.Setenv("HOME", tmpHome)

		if hasStatusLineConfigured() {
			t.Error("hasStatusLineConfigured() = true, want false when no settings")
		}
	})

	t.Run("returns false when statusLine not configured", func(t *testing.T) {
		tmpHome := t.TempDir()
		os.Setenv("HOME", tmpHome)

		claudeDir := filepath.Join(tmpHome, ".claude")
		os.MkdirAll(claudeDir, 0755)
		os.WriteFile(filepath.Join(claudeDir, "settings.json"),
			[]byte(`{"theme": "dark"}`), 0644)

		if hasStatusLineConfigured() {
			t.Error("hasStatusLineConfigured() = true, want false when statusLine absent")
		}
	})

	t.Run("returns false when statusLine has no command", func(t *testing.T) {
		tmpHome := t.TempDir()
		os.Setenv("HOME", tmpHome)

		claudeDir := filepath.Join(tmpHome, ".claude")
		os.MkdirAll(claudeDir, 0755)
		os.WriteFile(filepath.Join(claudeDir, "settings.json"),
			[]byte(`{"statusLine": {"type": "command"}}`), 0644)

		if hasStatusLineConfigured() {
			t.Error("hasStatusLineConfigured() = true, want false when command missing")
		}
	})

	t.Run("returns false when command is empty string", func(t *testing.T) {
		tmpHome := t.TempDir()
		os.Setenv("HOME", tmpHome)

		claudeDir := filepath.Join(tmpHome, ".claude")
		os.MkdirAll(claudeDir, 0755)
		os.WriteFile(filepath.Join(claudeDir, "settings.json"),
			[]byte(`{"statusLine": {"command": ""}}`), 0644)

		if hasStatusLineConfigured() {
			t.Error("hasStatusLineConfigured() = true, want false when command empty")
		}
	})

	t.Run("returns true when command is configured", func(t *testing.T) {
		tmpHome := t.TempDir()
		os.Setenv("HOME", tmpHome)

		claudeDir := filepath.Join(tmpHome, ".claude")
		os.MkdirAll(claudeDir, 0755)
		os.WriteFile(filepath.Join(claudeDir, "settings.json"),
			[]byte(`{"statusLine": {"command": "/path/to/script.sh"}}`), 0644)

		if !hasStatusLineConfigured() {
			t.Error("hasStatusLineConfigured() = false, want true when command set")
		}
	})
}

package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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

// TestHasStatusLineConfigured tests status line detection.
// Uses temp HOME to avoid affecting real user settings.
func TestHasStatusLineConfigured(t *testing.T) {
	// Save and restore HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	t.Run(
		"returns false when no settings file", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when no settings")
			}
		},
	)

	t.Run(
		"returns false when statusLine not configured", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"theme": "dark"}`), 0644,
			)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when statusLine absent")
			}
		},
	)

	t.Run(
		"returns false when statusLine has no command", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"statusLine": {"type": "command"}}`), 0644,
			)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when command missing")
			}
		},
	)

	t.Run(
		"returns false when command is empty string", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"statusLine": {"command": ""}}`), 0644,
			)

			if hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = true, want false when command empty")
			}
		},
	)

	t.Run(
		"returns true when command is configured", func(t *testing.T) {
			tmpHome := t.TempDir()
			os.Setenv("HOME", tmpHome)

			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)
			os.WriteFile(
				filepath.Join(claudeDir, "settings.json"),
				[]byte(`{"statusLine": {"command": "/path/to/script.sh"}}`), 0644,
			)

			if !hasStatusLineConfigured() {
				t.Error("hasStatusLineConfigured() = false, want true when command set")
			}
		},
	)
}

// TestHandlePromptNonGitRepo verifies graceful handling in non-git directories.
func TestHandlePromptNonGitRepo(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create and change to a non-git temp directory
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Capture stdout to verify no blocking output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	tests := []struct {
		name   string
		prompt string
	}{
		{"bumper-reset", "/bumper-reset"},
		{"bumper-pause", "/bumper-pause"},
		{"bumper-config", "/bumper-config"},
		{"bumper-tree", "/bumper-tree"},
		{"long form", "/claude-bumper-lanes:bumper-depth"},
		{"non-bumper prompt", "hello world"},
	}

	for _, tc := range tests {
		t.Run(
			tc.name, func(t *testing.T) {
				input := &HookInput{
					SessionID:  "test-session-123",
					UserPrompt: tc.prompt,
				}

				exitCode := HandlePrompt(input)

				if exitCode != 0 {
					t.Errorf("HandlePrompt(%q) = %d, want 0 (pass through)", tc.prompt, exitCode)
				}
			},
		)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "" {
		t.Errorf("HandlePrompt in non-git repo produced output: %q, want empty (pass through)", output)
	}
}

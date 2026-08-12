package hooks

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/claude-bumper-lanes/bumper-lanes-plugin/tools/bumper-lanes/internal/state"
)

// TestSessionStartPreservesStateOnCompactAndResume verifies that SessionStart
// events reusing an existing session id (compaction, resume) keep the baseline
// and score instead of re-baselining, and inject a budget recap.
func TestSessionStartPreservesStateOnCompactAndResume(t *testing.T) {
	// Isolate HOME: SessionStart's statusline setup must not see the real
	// ~/.claude (belt to the isTestProcess guard's suspenders).
	t.Setenv("HOME", t.TempDir())

	tmpDir := t.TempDir()
	setupTempGitRepo(t, tmpDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	os.WriteFile("initial.txt", []byte("initial\n"), 0644)
	exec.Command("git", "add", "initial.txt").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	sessionID := "test-preserve-state"
	baseline, _ := CaptureTree()

	seedState := func() {
		sess, err := state.New(sessionID, baseline, "main", 600)
		if err != nil {
			t.Fatalf("state.New: %v", err)
		}
		sess.SetScore(400)
		sess.SetStopTriggered(true)
		if err := sess.Save(); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	runSessionStart := func(source string) (int, string) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		exitCode := SessionStart(&HookInput{
			SessionID:     sessionID,
			HookEventName: "SessionStart",
			Source:        source,
		})
		w.Close()
		os.Stdout = oldStdout
		out, _ := io.ReadAll(r)
		return exitCode, string(out)
	}

	for _, source := range []string{"compact", "resume"} {
		t.Run(source+" preserves state and injects recap", func(t *testing.T) {
			seedState()
			exitCode, out := runSessionStart(source)
			if exitCode != 0 {
				t.Errorf("SessionStart(%s) = %d, want 0", source, exitCode)
			}

			sess, err := state.Load(sessionID)
			if err != nil {
				t.Fatalf("load after %s: %v", source, err)
			}
			if sess.BaselineTree != baseline {
				t.Errorf("baseline re-captured on %s: %s != %s", source, sess.BaselineTree, baseline)
			}
			if sess.Score != 400 {
				t.Errorf("score = %d after %s, want 400 (preserved)", sess.Score, source)
			}
			if !sess.StopTriggered {
				t.Errorf("stop_triggered cleared on %s, want preserved", source)
			}
			if !strings.Contains(out, "additionalContext") || !strings.Contains(out, "200/600 review-budget pts remain") {
				t.Errorf("recap missing from output on %s: %q", source, out)
			}
		})
	}

	t.Run("startup re-baselines as before", func(t *testing.T) {
		seedState()
		exitCode, _ := runSessionStart("startup")
		if exitCode != 0 && exitCode != 1 {
			t.Errorf("SessionStart(startup) = %d, want 0 or 1", exitCode)
		}
		sess, err := state.Load(sessionID)
		if err != nil {
			t.Fatalf("load after startup: %v", err)
		}
		if sess.Score != 0 {
			t.Errorf("score = %d after startup, want 0 (fresh baseline)", sess.Score)
		}
	})

	t.Run("compact without existing state falls through to fresh baseline", func(t *testing.T) {
		state.Delete(sessionID)
		exitCode, _ := runSessionStart("compact")
		if exitCode != 0 && exitCode != 1 {
			t.Errorf("SessionStart(compact, no state) = %d, want 0 or 1", exitCode)
		}
		if _, err := state.Load(sessionID); err != nil {
			t.Errorf("no state created on fall-through: %v", err)
		}
	})
}

func TestIsOurWrapper(t *testing.T) {
	// Create temp dir for test files
	tmpDir := t.TempDir()

	// Create a file with our marker
	markerFile := filepath.Join(tmpDir, "with-marker.sh")
	if err := os.WriteFile(markerFile, []byte("#!/bin/bash\n"+wrapperMarker+" - DO NOT EDIT\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file without our marker
	noMarkerFile := filepath.Join(tmpDir, "no-marker.sh")
	if err := os.WriteFile(noMarkerFile, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file with our exact wrapper filename
	wrapperFile := filepath.Join(tmpDir, wrapperFileName)
	if err := os.WriteFile(wrapperFile, []byte("#!/bin/bash\necho wrapper"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a fake binary named "bumper-lanes" that contains the marker
	// (simulates the compiled Go binary containing marker as string constant)
	binaryWithMarker := filepath.Join(tmpDir, "bumper-lanes")
	binaryContent := []byte{0x7f, 'E', 'L', 'F'} // ELF magic number
	binaryContent = append(binaryContent, []byte("garbage"+wrapperMarker+"more garbage")...)
	if err := os.WriteFile(binaryWithMarker, binaryContent, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a non-shebang script with marker (e.g., TypeScript statusline)
	nonShebangScript := filepath.Join(tmpDir, "statusline.ts")
	if err := os.WriteFile(nonShebangScript, []byte("// TypeScript\n"+wrapperMarker+"\nconsole.log('hi')"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"empty cmd", "", false},
		{"nonexistent file", "/nonexistent/path/script.sh", false},
		{"file with marker", markerFile, true},
		{"file without marker", noMarkerFile, false},
		{"wrapper filename match", wrapperFile, true},
		{"wrapper filename in different dir", filepath.Join("/some/other/path", wrapperFileName), true}, // filename match, file doesn't need to exist
		{"bumper-lanes binary with marker", binaryWithMarker, false},                                    // our binary contains marker, must not false-positive
		{"non-shebang script with marker", nonShebangScript, true},                                      // TypeScript etc. should still match
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOurWrapper(tt.cmd, tmpDir)
			if got != tt.want {
				t.Errorf("isOurWrapper(%q, %q) = %v, want %v", tt.cmd, tmpDir, got, tt.want)
			}
		})
	}
}

func TestGenerateWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	wrapperPath := filepath.Join(tmpDir, "test-wrapper.sh")
	originalCmd := "/usr/bin/my-status-line"

	err := generateWrapper(wrapperPath, originalCmd, tmpDir)
	if err != nil {
		t.Fatalf("generateWrapper() error = %v", err)
	}

	// Read generated wrapper
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("failed to read wrapper: %v", err)
	}

	contentStr := string(content)

	// Check marker is present
	if !contains(contentStr, wrapperMarker) {
		t.Error("wrapper missing marker")
	}

	// Check BUMPER_BIN marker is present
	if !contains(contentStr, "# BUMPER_BIN: ") {
		t.Error("wrapper missing BUMPER_BIN marker")
	}

	// Check original command is referenced
	if !contains(contentStr, originalCmd) {
		t.Error("wrapper missing original command")
	}

	// Check it's executable
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatalf("failed to stat wrapper: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Error("wrapper is not executable")
	}
}

func TestGetWrapperBinaryPath(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("extracts BUMPER_BIN from wrapper", func(t *testing.T) {
		wrapperPath := filepath.Join(tmpDir, "wrapper-with-bin.sh")
		content := `#!/bin/bash
# Generated by bumper-lanes - DO NOT EDIT
# BUMPER_BIN: /path/to/bumper-lanes
# Original command: /usr/bin/my-status
echo hello`
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}

		got := getWrapperBinaryPath(wrapperPath)
		want := "/path/to/bumper-lanes"
		if got != want {
			t.Errorf("getWrapperBinaryPath() = %q, want %q", got, want)
		}
	})

	t.Run("returns empty for missing marker", func(t *testing.T) {
		wrapperPath := filepath.Join(tmpDir, "wrapper-no-bin.sh")
		content := `#!/bin/bash
# Generated by bumper-lanes - DO NOT EDIT
echo hello`
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}

		got := getWrapperBinaryPath(wrapperPath)
		if got != "" {
			t.Errorf("getWrapperBinaryPath() = %q, want empty string", got)
		}
	})

	t.Run("returns empty for nonexistent file", func(t *testing.T) {
		got := getWrapperBinaryPath("/nonexistent/path.sh")
		if got != "" {
			t.Errorf("getWrapperBinaryPath() = %q, want empty string", got)
		}
	})
}

func TestGetOriginalCommand(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("extracts original command from wrapper", func(t *testing.T) {
		wrapperPath := filepath.Join(tmpDir, "wrapper.sh")
		content := `#!/bin/bash
# Generated by bumper-lanes - DO NOT EDIT
# BUMPER_BIN: /path/to/bumper-lanes
# Original command: /usr/bin/my-custom-status
echo hello`
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}

		got := getOriginalCommand(wrapperPath)
		want := "/usr/bin/my-custom-status"
		if got != want {
			t.Errorf("getOriginalCommand() = %q, want %q", got, want)
		}
	})

	t.Run("returns empty for missing marker", func(t *testing.T) {
		wrapperPath := filepath.Join(tmpDir, "wrapper-no-orig.sh")
		content := `#!/bin/bash
echo hello`
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}

		got := getOriginalCommand(wrapperPath)
		if got != "" {
			t.Errorf("getOriginalCommand() = %q, want empty string", got)
		}
	})
}

func TestIsWrapperStale(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("stale when BUMPER_BIN differs from current", func(t *testing.T) {
		wrapperPath := filepath.Join(tmpDir, "stale-wrapper.sh")
		// Use a path that definitely won't match the current binary
		content := `#!/bin/bash
# Generated by bumper-lanes - DO NOT EDIT
# BUMPER_BIN: /old/path/to/bumper-lanes
# Original command: /usr/bin/my-status
echo hello`
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}

		if !isWrapperStale(wrapperPath) {
			t.Error("isWrapperStale() = false, want true for different path")
		}
	})

	t.Run("stale when BUMPER_BIN marker missing (old format)", func(t *testing.T) {
		wrapperPath := filepath.Join(tmpDir, "old-format-wrapper.sh")
		content := `#!/bin/bash
# Generated by bumper-lanes - DO NOT EDIT
# Original command: /usr/bin/my-status
echo hello`
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}

		if !isWrapperStale(wrapperPath) {
			t.Error("isWrapperStale() = false, want true for missing BUMPER_BIN marker")
		}
	})

	t.Run("not stale when BUMPER_BIN matches current", func(t *testing.T) {
		wrapperPath := filepath.Join(tmpDir, "current-wrapper.sh")
		currentBin := getBumperLanesBinPath()
		content := `#!/bin/bash
# Generated by bumper-lanes - DO NOT EDIT
# BUMPER_BIN: ` + currentBin + `
# Original command: /usr/bin/my-status
echo hello`
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}

		if isWrapperStale(wrapperPath) {
			t.Errorf("isWrapperStale() = true, want false when BUMPER_BIN matches current (%s)", currentBin)
		}
	})
}

func TestIsHandsOff(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("returns true for script with marker", func(t *testing.T) {
		withMarker := filepath.Join(tmpDir, "with-marker.sh")
		if err := os.WriteFile(withMarker, []byte("#!/bin/bash\n# BUMPER_HANDS_OFF\necho hi"), 0755); err != nil {
			t.Fatal(err)
		}
		if !isHandsOff(withMarker) {
			t.Error("isHandsOff() = false, want true for script with marker")
		}
	})

	t.Run("returns false for script without marker", func(t *testing.T) {
		withoutMarker := filepath.Join(tmpDir, "without-marker.sh")
		if err := os.WriteFile(withoutMarker, []byte("#!/bin/bash\necho hi"), 0755); err != nil {
			t.Fatal(err)
		}
		if isHandsOff(withoutMarker) {
			t.Error("isHandsOff() = true, want false for script without marker")
		}
	})

	t.Run("returns false for non-existent file", func(t *testing.T) {
		if isHandsOff("/nonexistent/path") {
			t.Error("isHandsOff() = true, want false for non-existent file")
		}
	})

	t.Run("returns false for empty path", func(t *testing.T) {
		if isHandsOff("") {
			t.Error("isHandsOff() = true, want false for empty path")
		}
	})

	t.Run("returns false for bumper-lanes binary", func(t *testing.T) {
		// The compiled bumper-lanes binary contains the marker as a string constant.
		// isHandsOff must not false-positive match on our own binary.
		binaryPath := filepath.Join(tmpDir, "bumper-lanes")
		binaryContent := []byte{0x7f, 'E', 'L', 'F'} // ELF magic number
		binaryContent = append(binaryContent, []byte("garbage"+handsOffMarker+"more garbage")...)
		if err := os.WriteFile(binaryPath, binaryContent, 0755); err != nil {
			t.Fatal(err)
		}
		if isHandsOff(binaryPath) {
			t.Error("isHandsOff() = true, want false for bumper-lanes binary")
		}
	})

	t.Run("returns true for non-shebang script with marker", func(t *testing.T) {
		// TypeScript, Node.js, etc. may not have shebangs
		tsScript := filepath.Join(tmpDir, "statusline.ts")
		if err := os.WriteFile(tsScript, []byte("// TypeScript statusline\n// # BUMPER_HANDS_OFF\nconsole.log('hi')"), 0755); err != nil {
			t.Fatal(err)
		}
		if !isHandsOff(tsScript) {
			t.Error("isHandsOff() = false, want true for non-shebang script with marker")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

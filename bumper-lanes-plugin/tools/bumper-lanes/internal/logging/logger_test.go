package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerHygiene(t *testing.T) {
	// This test binary is itself a go-test process, so New must route away
	// from the operator directory and prune old files in the target dir.
	testLogDir := filepath.Join(os.TempDir(), "bumper-lanes-test-logs")
	os.MkdirAll(testLogDir, 0755)
	stale := filepath.Join(testLogDir, "2020-01-01-session-ancient.log")
	os.WriteFile(stale, []byte("old\n"), 0644)
	old := time.Now().AddDate(0, 0, -40)
	os.Chtimes(stale, old, old)

	logger := New("hygiene-check", "test")

	if !IsTestProcess() {
		t.Fatal("a go-test binary must report IsTestProcess")
	}
	if !strings.HasPrefix(logger.LogFile(), testLogDir) {
		t.Errorf("test-process log = %s, want under %s", logger.LogFile(), testLogDir)
	}
	wantName := time.Now().Format("2006-01-02") + "-session-hygiene-check.log"
	if filepath.Base(logger.LogFile()) != wantName {
		t.Errorf("log name = %s, want %s", filepath.Base(logger.LogFile()), wantName)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("40-day-old log survived the retention prune")
	}

	logger.Info("hello")
	if data, err := os.ReadFile(logger.LogFile()); err != nil || !strings.Contains(string(data), "hello") {
		t.Errorf("log write failed: err=%v content=%q", err, data)
	}
}

// readLog reads the logger's file and fails the test if it can't.
func readLog(t *testing.T, logger *Logger) string {
	t.Helper()
	data, err := os.ReadFile(logger.LogFile())
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	return string(data)
}

func TestLoggerLevels(t *testing.T) {
	t.Run("Info, Warn, Error all reach the file with their level tag", func(t *testing.T) {
		logger := New(t.Name(), "test")
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")

		got := readLog(t, logger)
		for _, want := range []string{"[INFO] [test] info message", "[WARN] [test] warn message", "[ERROR] [test] error message"} {
			if !strings.Contains(got, want) {
				t.Errorf("log = %q, want it to contain %q", got, want)
			}
		}
	})

	t.Run("Debug is dropped unless BUMPER_LANES_DEBUG=1 was set at construction", func(t *testing.T) {
		// Clear ambient env explicitly: New reads BUMPER_LANES_DEBUG from
		// the real environment, so a value left set in the shell running
		// the tests (not just a prior subtest) would leak into this
		// assertion.
		t.Setenv("BUMPER_LANES_DEBUG", "")
		logger := New(t.Name(), "test")
		// The log file name is date+session-derived, so a run earlier
		// today (e.g. with BUMPER_LANES_DEBUG genuinely set) can leave a
		// debug line in the same path; start from a fresh, empty file.
		os.Remove(logger.LogFile())
		logger.Debug("debug message")

		if _, err := os.Stat(logger.LogFile()); err == nil {
			got := readLog(t, logger)
			if strings.Contains(got, "debug message") {
				t.Errorf("log = %q, debug message should have been filtered", got)
			}
		}
	})

	t.Run("Debug is kept when BUMPER_LANES_DEBUG=1 was set before New", func(t *testing.T) {
		t.Setenv("BUMPER_LANES_DEBUG", "1")
		logger := New(t.Name(), "test")
		logger.Debug("debug message")

		got := readLog(t, logger)
		if !strings.Contains(got, "[DEBUG] [test] debug message") {
			t.Errorf("log = %q, want it to contain the debug message", got)
		}
	})
}

func TestLoggerMultilineMessage(t *testing.T) {
	logger := New(t.Name(), "test")
	logger.Info("line one\nline two")

	got := readLog(t, logger)
	if !strings.Contains(got, "[INFO] [test]\nline one\nline two\n") {
		t.Errorf("log = %q, want the multiline message on its own line", got)
	}
}

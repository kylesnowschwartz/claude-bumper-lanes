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

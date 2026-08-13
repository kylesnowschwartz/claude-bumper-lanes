// Package logging provides session-based file logging for bumper-lanes hooks.
// Logs are written to ~/.claude/logs/bumper-lanes/{date}-session-{session_id}.log.
// Test processes (go test binaries) log under the system temp directory
// instead, so the operator-facing directory only ever holds real sessions;
// files older than retentionDays are pruned once per process.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Level represents log severity.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger handles session-based file logging, built on log/slog.
type Logger struct {
	slog    *slog.Logger
	logFile string
}

// sessionIDSanitizer replaces non-alphanumeric chars (except - and _) with _
var sessionIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

// New creates a logger for the given session and source component. It reads
// BUMPER_LANES_DEBUG to decide the level at construction time, so tests can
// control it per-logger via t.Setenv before calling New.
func New(sessionID, source string) *Logger {
	safeID := sanitizeSessionID(sessionID)
	logDir := getLogDir()
	pruneOnce.Do(func() { pruneOldLogs(logDir) })
	logFile := filepath.Join(logDir, fmt.Sprintf("%s-session-%s.log", time.Now().Format("2006-01-02"), safeID))

	level := LevelInfo
	if os.Getenv("BUMPER_LANES_DEBUG") == "1" {
		level = LevelDebug
	}

	handler := &fileHandler{
		path:   logFile,
		source: source,
		level:  level,
	}

	return &Logger{
		slog:    slog.New(handler),
		logFile: logFile,
	}
}

// IsTestProcess reports whether this binary is a go-test artifact. Shared
// by every guard that must not touch user-global state from a test run
// (log directory here, statusline wrapper in the session-start hook).
func IsTestProcess() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.HasSuffix(exe, ".test") || strings.Contains(exe, string(filepath.Separator)+"go-build")
}

// retentionDays bounds how long session logs are kept.
const retentionDays = 30

var pruneOnce sync.Once

// pruneOldLogs removes log files past retention. Best-effort: never blocks
// or fails the operation being logged.
func pruneOldLogs(logDir string) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logDir, entry.Name()))
		}
	}
}

// Debug logs a debug message (only if BUMPER_LANES_DEBUG=1 was set when the
// logger was created).
func (l *Logger) Debug(format string, args ...any) { l.log(LevelDebug, format, args...) }

// Info logs an info message.
func (l *Logger) Info(format string, args ...any) { l.log(LevelInfo, format, args...) }

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...any) { l.log(LevelWarn, format, args...) }

// Error logs an error message.
func (l *Logger) Error(format string, args ...any) { l.log(LevelError, format, args...) }

// log formats the printf-style message and hands it to slog, which applies
// the level filter and the fileHandler's line format.
func (l *Logger) log(level Level, format string, args ...any) {
	if !l.slog.Enabled(context.Background(), level) {
		return
	}
	l.slog.Log(context.Background(), level, fmt.Sprintf(format, args...))
}

// getLogDir returns the log directory path (~/.claude/logs/bumper-lanes).
// Test processes get a temp-dir sibling so go test runs never write into
// the operator-facing directory.
func getLogDir() string {
	if IsTestProcess() {
		return filepath.Join(os.TempDir(), "bumper-lanes-test-logs")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to /tmp if home dir unavailable
		return filepath.Join(os.TempDir(), "bumper-lanes-logs")
	}
	return filepath.Join(homeDir, ".claude", "logs", "bumper-lanes")
}

// sanitizeSessionID makes session ID filesystem-safe
func sanitizeSessionID(sessionID string) string {
	if sessionID == "" {
		return "unknown"
	}
	return sessionIDSanitizer.ReplaceAllString(sessionID, "_")
}

// LogFile returns the path to the current log file
func (l *Logger) LogFile() string {
	return l.logFile
}

// fileHandler is a slog.Handler that appends one line per record to a
// session log file: "[timestamp] [LEVEL] [source] message", with the
// message moved to its own line when it contains a newline. It opens and
// closes the file on every write instead of holding it open, and relies on
// the OS write buffer instead of fsyncing each line, keeping a per-hook-
// invocation write affordable. If the file can't be opened or written, the
// entry falls back to stderr.
type fileHandler struct {
	mu     sync.Mutex
	path   string
	source string
	level  Level
}

func (h *fileHandler) Enabled(_ context.Context, level Level) bool {
	return level >= h.level
}

func (h *fileHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	timestamp := r.Time.Format("2006-01-02 15:04:05")
	var entry string
	if strings.Contains(r.Message, "\n") {
		entry = fmt.Sprintf("[%s] [%s] [%s]\n%s\n", timestamp, r.Level, h.source, r.Message)
	} else {
		entry = fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, r.Level, h.source, r.Message)
	}

	if err := h.writeToFile(entry); err != nil {
		fmt.Fprintf(os.Stderr, "bumper-lanes: logging failed: %v\n", err)
		fmt.Fprint(os.Stderr, entry)
	}
	return nil
}

func (h *fileHandler) writeToFile(entry string) error {
	if err := os.MkdirAll(filepath.Dir(h.path), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}
	return nil
}

func (h *fileHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *fileHandler) WithGroup(_ string) slog.Handler      { return h }

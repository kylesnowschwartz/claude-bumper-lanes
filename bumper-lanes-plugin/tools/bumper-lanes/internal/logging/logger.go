// Package logging provides session-based file logging for bumper-lanes hooks.
// Logs are written to ~/.claude/logs/bumper-lanes/session-{session_id}.log
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Level represents log severity
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Logger handles session-based file logging
type Logger struct {
	sessionID string
	source    string
	logFile   string
	mu        sync.Mutex
}

var (
	// sessionIDSanitizer replaces non-alphanumeric chars (except - and _) with _
	sessionIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

	// debugEnabled is set by BUMPER_LANES_DEBUG=1
	debugEnabled = os.Getenv("BUMPER_LANES_DEBUG") == "1"
)

// New creates a logger for the given session and source component
func New(sessionID, source string) *Logger {
	safeID := sanitizeSessionID(sessionID)
	logDir := getLogDir()
	logFile := filepath.Join(logDir, fmt.Sprintf("session-%s.log", safeID))

	return &Logger{
		sessionID: sessionID,
		source:    source,
		logFile:   logFile,
	}
}

// Debug logs a debug message (only if BUMPER_LANES_DEBUG=1)
func (l *Logger) Debug(format string, args ...interface{}) {
	if debugEnabled {
		l.log(LevelDebug, format, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// log writes a log entry to the session log file
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	var entry string
	if strings.Contains(message, "\n") {
		// Multiline: put message on new line
		entry = fmt.Sprintf("[%s] [%s] [%s]\n%s\n", timestamp, level, l.source, message)
	} else {
		entry = fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, level, l.source, message)
	}

	if err := l.writeToFile(entry); err != nil {
		// Fallback to stderr if file logging fails
		fmt.Fprintf(os.Stderr, "bumper-lanes: logging failed: %v\n", err)
		fmt.Fprint(os.Stderr, entry)
	}
}

// writeToFile appends the entry to the log file
func (l *Logger) writeToFile(entry string) error {
	// Ensure log directory exists
	logDir := filepath.Dir(l.logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file in append mode (thread-safe via mutex)
	f, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	// Immediate flush
	return f.Sync()
}

// getLogDir returns the log directory path (~/.claude/logs/bumper-lanes)
func getLogDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to /tmp if home dir unavailable
		return "/tmp/bumper-lanes-logs"
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

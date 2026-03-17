package limitless

import (
	"fmt"
	"os"
)

// Logger defines the logging interface for the SDK.
type Logger interface {
	Debug(msg string, meta ...map[string]any)
	Info(msg string, meta ...map[string]any)
	Warn(msg string, meta ...map[string]any)
	Error(msg string, err error, meta ...map[string]any)
}

// noopLogger is the default logger that does nothing.
type noopLogger struct{}

func (noopLogger) Debug(string, ...map[string]any)        {}
func (noopLogger) Info(string, ...map[string]any)         {}
func (noopLogger) Warn(string, ...map[string]any)         {}
func (noopLogger) Error(string, error, ...map[string]any) {}

// NewNoOpLogger returns a logger that discards all output.
func NewNoOpLogger() Logger { return noopLogger{} }

// LogLevel represents the minimum log level for the ConsoleLogger.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// ConsoleLogger prints log messages to stderr with a level prefix.
type ConsoleLogger struct {
	Level LogLevel
}

// NewConsoleLogger creates a new console logger with the given minimum level.
func NewConsoleLogger(level LogLevel) *ConsoleLogger {
	return &ConsoleLogger{Level: level}
}

func (l *ConsoleLogger) shouldLog(level LogLevel) bool {
	return level >= l.Level
}

func (l *ConsoleLogger) Debug(msg string, meta ...map[string]any) {
	if l.shouldLog(LogLevelDebug) {
		fmt.Fprintf(os.Stderr, "[Limitless SDK] DEBUG %s %v\n", msg, metaStr(meta))
	}
}

func (l *ConsoleLogger) Info(msg string, meta ...map[string]any) {
	if l.shouldLog(LogLevelInfo) {
		fmt.Fprintf(os.Stderr, "[Limitless SDK] INFO  %s %v\n", msg, metaStr(meta))
	}
}

func (l *ConsoleLogger) Warn(msg string, meta ...map[string]any) {
	if l.shouldLog(LogLevelWarn) {
		fmt.Fprintf(os.Stderr, "[Limitless SDK] WARN  %s %v\n", msg, metaStr(meta))
	}
}

func (l *ConsoleLogger) Error(msg string, err error, meta ...map[string]any) {
	if l.shouldLog(LogLevelError) {
		errMsg := ""
		if err != nil {
			errMsg = " - " + err.Error()
		}
		fmt.Fprintf(os.Stderr, "[Limitless SDK] ERROR %s%s %v\n", msg, errMsg, metaStr(meta))
	}
}

func metaStr(meta []map[string]any) string {
	if len(meta) == 0 || meta[0] == nil {
		return ""
	}
	return fmt.Sprintf("%v", meta[0])
}

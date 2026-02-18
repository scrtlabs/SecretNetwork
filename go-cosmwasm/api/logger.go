package api

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

var (
	globalLogger     *logger
	loggerOnce       sync.Once
	defaultLogLevel  = LogLevelInfo
	logLevelEnvVar   = "LOG_LEVEL"
)

type logger struct {
	level LogLevel
}

// getLogger returns the global logger instance
func getLogger() *logger {
	loggerOnce.Do(func() {
		level := parseLogLevel(os.Getenv(logLevelEnvVar))
		if level == -1 {
			level = defaultLogLevel
		}
		globalLogger = &logger{level: level}
	})
	return globalLogger
}

// parseLogLevel parses a log level string (case-insensitive)
func parseLogLevel(s string) LogLevel {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "ERROR":
		return LogLevelError
	case "WARN", "WARNING":
		return LogLevelWarn
	case "INFO":
		return LogLevelInfo
	case "DEBUG":
		return LogLevelDebug
	default:
		return -1
	}
}

// shouldLog returns true if the given level should be logged
func (l *logger) shouldLog(level LogLevel) bool {
	return level <= l.level
}

// logf formats and prints a log message if the level is enabled
func (l *logger) logf(level LogLevel, prefix, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", prefix, msg)
}

// Debug logs a debug message
func logDebug(prefix, format string, args ...interface{}) {
	getLogger().logf(LogLevelDebug, prefix, format, args...)
}

// Info logs an info message
func logInfo(prefix, format string, args ...interface{}) {
	getLogger().logf(LogLevelInfo, prefix, format, args...)
}

// Warn logs a warning message
func logWarn(prefix, format string, args ...interface{}) {
	getLogger().logf(LogLevelWarn, prefix, format, args...)
}

// Error logs an error message
func logError(prefix, format string, args ...interface{}) {
	getLogger().logf(LogLevelError, prefix, format, args...)
}


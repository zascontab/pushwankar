package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of the log message
type LogLevel int

const (
	// DEBUG level for detailed information (mostly useful for development)
	DEBUG LogLevel = iota
	// INFO level for general operational information
	INFO
	// WARN level for non-critical issues that might need attention
	WARN
	// ERROR level for error events that might still allow the application to continue
	ERROR
	// FATAL level for critical errors that prevent features from working
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Color returns the ANSI color code for the log level
func (l LogLevel) Color() string {
	switch l {
	case DEBUG:
		return "\033[36m" // Cyan
	case INFO:
		return "\033[32m" // Green
	case WARN:
		return "\033[33m" // Yellow
	case ERROR:
		return "\033[31m" // Red
	case FATAL:
		return "\033[35m" // Magenta
	default:
		return "\033[0m" // No color
	}
}

// Logger represents a logger instance
type Logger struct {
	level     LogLevel
	output    io.Writer
	prefix    string
	useColors bool
	mu        sync.Mutex
}

// LoggerOption is a function that configures a Logger
type LoggerOption func(*Logger)

// NewLogger creates a new logger with the specified options
func NewLogger(options ...LoggerOption) *Logger {
	// Default configuration
	logger := &Logger{
		level:     INFO,
		output:    os.Stdout,
		prefix:    "",
		useColors: true,
	}

	// Apply options
	for _, option := range options {
		option(logger)
	}

	return logger
}

// WithLevel sets the minimum log level
func WithLevel(level LogLevel) LoggerOption {
	return func(l *Logger) {
		l.level = level
	}
}

// WithOutput sets the output writer
func WithOutput(output io.Writer) LoggerOption {
	return func(l *Logger) {
		l.output = output
	}
}

// WithPrefix sets a prefix for all log messages
func WithPrefix(prefix string) LoggerOption {
	return func(l *Logger) {
		l.prefix = prefix
	}
}

// WithColors enables or disables colored output
func WithColors(useColors bool) LoggerOption {
	return func(l *Logger) {
		l.useColors = useColors
	}
}

// SetLevel changes the logger's minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// log logs a message with the specified level and optional formatting arguments
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	callerInfo := "??:??"
	if ok {
		callerInfo = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	// Format the message
	var msg string
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	} else {
		msg = format
	}

	// Build log line
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	var logLine string

	if l.useColors {
		reset := "\033[0m"
		levelStr := fmt.Sprintf("%s%s%s", level.Color(), level.String(), reset)
		prefix := ""
		if l.prefix != "" {
			prefix = fmt.Sprintf("%s[%s]%s ", level.Color(), l.prefix, reset)
		}
		logLine = fmt.Sprintf("%s [%s] %s%s %s\n", timestamp, levelStr, prefix, callerInfo, msg)
	} else {
		levelStr := level.String()
		prefix := ""
		if l.prefix != "" {
			prefix = fmt.Sprintf("[%s] ", l.prefix)
		}
		logLine = fmt.Sprintf("%s [%s] %s%s %s\n", timestamp, levelStr, prefix, callerInfo, msg)
	}

	// Write the log line
	fmt.Fprint(l.output, logLine)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
	os.Exit(1)
}

// ParseLevel parses a string into a LogLevel
func ParseLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// Default logger instance
var DefaultLogger = NewLogger()

// SetDefaultLogger sets the default logger
func SetDefaultLogger(logger *Logger) {
	DefaultLogger = logger
}

// Helper functions that use the default logger

// Debug logs a debug message to the default logger
func Debug(format string, args ...interface{}) {
	DefaultLogger.Debug(format, args...)
}

// Info logs an info message to the default logger
func Info(format string, args ...interface{}) {
	DefaultLogger.Info(format, args...)
}

// Warn logs a warning message to the default logger
func Warn(format string, args ...interface{}) {
	DefaultLogger.Warn(format, args...)
}

// Error logs an error message to the default logger
func Error(format string, args ...interface{}) {
	DefaultLogger.Error(format, args...)
}

// Fatal logs a fatal message to the default logger and exits
func Fatal(format string, args ...interface{}) {
	DefaultLogger.Fatal(format, args...)
}



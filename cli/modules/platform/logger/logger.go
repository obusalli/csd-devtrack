package logger

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"csd-devtrack/cli/modules/ui/core"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	default:
		return "info"
	}
}

// ParseLevel parses a log level string
func ParseLevel(level string) Level {
	switch strings.ToLower(level) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// LogBroadcaster is an interface for broadcasting log lines
type LogBroadcaster interface {
	BroadcastLog(line core.LogLineVM)
}

// Logger is the main logger
type Logger struct {
	mu          sync.Mutex
	level       Level
	outputs     []io.Writer
	broadcaster LogBroadcaster
	source      string
}

// NewLogger creates a new logger
func NewLogger(level Level, outputs []io.Writer, source string) *Logger {
	return &Logger{
		level:   level,
		outputs: outputs,
		source:  source,
	}
}

// SetBroadcaster sets the log broadcaster (daemon server)
func (l *Logger) SetBroadcaster(b LogBroadcaster) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.broadcaster = b
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) log(level Level, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	now := time.Now()
	timestamp := now.Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s: %s\n", timestamp, strings.ToUpper(level.String()), message)

	// Write to all configured outputs (console, file)
	for _, output := range l.outputs {
		output.Write([]byte(logMessage))
	}

	// Broadcast to connected clients
	if l.broadcaster != nil {
		logLine := core.LogLineVM{
			Timestamp: now,
			TimeStr:   now.Format("15:04:05"),
			Source:    l.source,
			Level:     level.String(),
			Message:   message,
		}
		l.broadcaster.BroadcastLog(logLine)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if len(args) == 0 {
		l.log(DEBUG, format)
	} else {
		l.log(DEBUG, fmt.Sprintf(format, args...))
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	if len(args) == 0 {
		l.log(INFO, format)
	} else {
		l.log(INFO, fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	if len(args) == 0 {
		l.log(WARN, format)
	} else {
		l.log(WARN, fmt.Sprintf(format, args...))
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	if len(args) == 0 {
		l.log(ERROR, format)
	} else {
		l.log(ERROR, fmt.Sprintf(format, args...))
	}
}

// CreateLogFile creates and returns a file writer for logging
func CreateLogFile(logPath string, maxSizeMB int) (*os.File, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open or create log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Check file size and rotate if necessary
	info, err := file.Stat()
	if err == nil && info.Size() > int64(maxSizeMB*1024*1024) {
		file.Close()
		rotateLog(logPath)
		file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file after rotation: %w", err)
		}
	}

	return file, nil
}

func rotateLog(logPath string) {
	timestamp := time.Now().Format("20060102-150405")
	newPath := fmt.Sprintf("%s.%s", logPath, timestamp)
	os.Rename(logPath, newPath)
}

// Global logger instance
var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(l *Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger == nil {
		// Return a default logger if not initialized
		return NewLogger(INFO, []io.Writer{os.Stderr}, "csd-devtrack")
	}
	return globalLogger
}

// Global logging functions for convenience
func Info(format string, args ...interface{}) {
	GetGlobalLogger().Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	GetGlobalLogger().Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	GetGlobalLogger().Error(format, args...)
}

func Debug(format string, args ...interface{}) {
	GetGlobalLogger().Debug(format, args...)
}

// CaptureStdout captures os.Stdout and redirects it through the logger
func CaptureStdout(l *Logger) (*os.File, error) {
	return captureOutput(l, &os.Stdout, INFO)
}

// CaptureStderr captures os.Stderr and redirects it through the logger
func CaptureStderr(l *Logger) (*os.File, error) {
	return captureOutput(l, &os.Stderr, ERROR)
}

func captureOutput(l *Logger, target **os.File, level Level) (*os.File, error) {
	// Save original
	original := *target

	// Create pipe
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	// Replace target with write end of pipe
	*target = w

	// Start goroutine to read from pipe and log
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// Detect level from content
			detectedLevel := level
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
				detectedLevel = ERROR
			} else if strings.Contains(lower, "warn") {
				detectedLevel = WARN
			} else if strings.Contains(lower, "debug") {
				detectedLevel = DEBUG
			}

			// Log through the logger (this will broadcast to clients)
			l.logDirect(detectedLevel, line, original)
		}
	}()

	return original, nil
}

// logDirect logs without writing to the captured output (to avoid loops)
func (l *Logger) logDirect(level Level, message string, directOutput io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	now := time.Now()
	timestamp := now.Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s: %s\n", timestamp, strings.ToUpper(level.String()), message)

	// Write only to the direct output (original stdout/stderr) and file outputs
	if directOutput != nil {
		directOutput.Write([]byte(logMessage))
	}

	// Write to file outputs (skip the first one if it's the captured stream)
	for _, output := range l.outputs {
		if output != os.Stdout && output != os.Stderr {
			output.Write([]byte(logMessage))
		}
	}

	// Broadcast to connected clients
	if l.broadcaster != nil {
		logLine := core.LogLineVM{
			Timestamp: now,
			TimeStr:   now.Format("15:04:05"),
			Source:    l.source,
			Level:     level.String(),
			Message:   message,
		}
		l.broadcaster.BroadcastLog(logLine)
	}
}

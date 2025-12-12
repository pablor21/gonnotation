package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"sync"
)

// LogLevel defines the logging verbosity
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelNone  LogLevel = "none"
)

var (
	currentLogTag string
	logTagMutex   sync.RWMutex
)

// Logger is the interface for logging during generation
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// simpleHandler is a simple log handler that outputs standard log format
type simpleHandler struct {
	level slog.Level
	w     io.Writer
}

func (h *simpleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *simpleHandler) Handle(ctx context.Context, r slog.Record) error {
	timeStr := r.Time.Format("2006/01/02 15:04:05")
	level := r.Level.String()

	// Get tag from global variable
	logTagMutex.RLock()
	tag := currentLogTag
	logTagMutex.RUnlock()

	if tag == "" {
		tag = "CORE"
	}

	_, err := fmt.Fprintf(h.w, "%s [%s] %s %s\n", timeStr, tag, level, r.Message)
	return err
}

func (h *simpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *simpleHandler) WithGroup(name string) slog.Handler {
	return h
}

// SetupLogger configures the global logger based on the log level
func SetupLogger(level LogLevel) {
	var slogLevel slog.Level

	switch level {
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	case LogLevelInfo:
		slogLevel = slog.LevelInfo
	case LogLevelWarn:
		slogLevel = slog.LevelWarn
	case LogLevelError:
		slogLevel = slog.LevelError
	case LogLevelNone:
		// Set to a very high level to suppress all logs
		slogLevel = slog.Level(1000)
	default:
		slogLevel = slog.LevelInfo
	}

	// Use simple handler for cleaner output
	handler := &simpleHandler{
		level: slogLevel,
		w:     os.Stderr,
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Also set the standard log package to use the same output
	log.SetOutput(os.Stderr)
	log.SetFlags(0)
}

// // GetEffectiveLogLevel determines the effective log level with format generator override
// func GetEffectiveLogLevel(config *Config, pluginConfig any) LogLevel {
// 	// Start with core config
// 	logLevel := utils.DerefPtr(config.LogLevel, LogLevelInfo)
// 	if logLevel == "" {
// 		logLevel = LogLevelInfo
// 	}

// 	// Check for format generator-level override
// 	if configMap, ok := pluginConfig.(map[string]any); ok {
// 		if pluginLevel, exists := configMap["log_level"]; exists {
// 			if levelStr, ok := pluginLevel.(string); ok {
// 				logLevel = LogLevel(levelStr)
// 			}
// 		}
// 	}

// 	return logLevel
// }

// SetLogTag sets the current log tag for all subsequent logs
func SetLogTag(tag string) {
	logTagMutex.Lock()
	currentLogTag = tag
	logTagMutex.Unlock()
}

// GetLogTag returns the current log tag
func GetLogTag() string {
	logTagMutex.RLock()
	defer logTagMutex.RUnlock()
	return currentLogTag
}

// DefaultLogger implements Logger using slog
type DefaultLogger struct{}

func NewDefaultLogger() Logger {
	return &DefaultLogger{}
}

func (l *DefaultLogger) Debug(msg string) {
	slog.Debug(msg)
}

func (l *DefaultLogger) Info(msg string) {
	slog.Info(msg)
}

func (l *DefaultLogger) Warn(msg string) {
	slog.Warn(msg)
}

func (l *DefaultLogger) Error(msg string) {
	slog.Error(msg)
}

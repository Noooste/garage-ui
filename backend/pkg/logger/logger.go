package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger is a wrapper around zerolog.Logger
type Logger struct {
	zerolog.Logger
}

var (
	// Global logger instance
	globalLogger *Logger
)

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Pretty     bool   // Enable console pretty printing
	TimeFormat string // Time format (default: time.RFC3339)
}

// Init initializes the global logger with the given configuration
func Init(cfg Config) {
	var output io.Writer = os.Stdout

	// Set up pretty console output if enabled
	if cfg.Pretty {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	// Parse log level
	level := zerolog.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}

	// Create logger
	logger := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	globalLogger = &Logger{logger}
	log.Logger = logger
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		// Initialize with defaults if not initialized
		Init(Config{
			Level:  "info",
			Pretty: true,
		})
	}
	return globalLogger
}

// Debug logs a debug message
func Debug() *zerolog.Event {
	return Get().Debug()
}

// Info logs an info message
func Info() *zerolog.Event {
	return Get().Info()
}

// Warn logs a warning message
func Warn() *zerolog.Event {
	return Get().Warn()
}

// Error logs an error message
func Error() *zerolog.Event {
	return Get().Error()
}

// Fatal logs a fatal message and exits
func Fatal() *zerolog.Event {
	return Get().Fatal()
}

// WithContext creates a new logger with additional context fields
func (l *Logger) WithContext(fields map[string]interface{}) *Logger {
	ctx := l.Logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &Logger{ctx.Logger()}
}

// WithComponent creates a logger with a component field
func WithComponent(component string) *Logger {
	return &Logger{Get().With().Str("component", component).Logger()}
}

// WithError creates a logger with an error field
func WithError(err error) *zerolog.Event {
	return Get().Error().Err(err)
}

package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.SugaredLogger for application-wide logging
type Logger struct {
	*zap.SugaredLogger
}

var (
	// Global logger instance
	globalLogger *Logger
)

// NewLogger creates a new logger with the specified level
func NewLogger(level string, development bool) (*Logger, error) {
	var config zap.Config

	if development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.Encoding = "json"
	}

	// Parse and set log level
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(zapLevel)

	// Build logger
	baseLogger, err := config.Build(
		zap.AddCallerSkip(1), // Skip one level to show correct caller
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return &Logger{
		SugaredLogger: baseLogger.Sugar(),
	}, nil
}

// NewProductionLogger creates a production logger (JSON, info level)
func NewProductionLogger() (*Logger, error) {
	return NewLogger("info", false)
}

// NewDevelopmentLogger creates a development logger (console, debug level)
func NewDevelopmentLogger() (*Logger, error) {
	return NewLogger("debug", true)
}

// InitGlobalLogger initializes the global logger
func InitGlobalLogger(level string, development bool) error {
	logger, err := NewLogger(level, development)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		// Create default production logger if not initialized
		logger, err := NewProductionLogger()
		if err != nil {
			// Fallback to nop logger
			globalLogger = &Logger{SugaredLogger: zap.NewNop().Sugar()}
		} else {
			globalLogger = logger
		}
	}
	return globalLogger
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.SugaredLogger.Sync()
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields ...interface{}) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.With(fields...),
	}
}

// WithError returns a logger with an error field
func (l *Logger) WithError(err error) *Logger {
	return l.WithFields("error", err.Error())
}

// Helper functions using global logger
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	GetLogger().Debugf(template, args...)
}

func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

func Infof(template string, args ...interface{}) {
	GetLogger().Infof(template, args...)
}

func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	GetLogger().Warnf(template, args...)
}

func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

func Errorf(template string, args ...interface{}) {
	GetLogger().Errorf(template, args...)
}

func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
	os.Exit(1)
}

func Fatalf(template string, args ...interface{}) {
	GetLogger().Fatalf(template, args...)
	os.Exit(1)
}

// WithFields returns a global logger with additional fields
func WithFields(fields ...interface{}) *Logger {
	return GetLogger().WithFields(fields...)
}

// WithError returns a global logger with an error field
func WithError(err error) *Logger {
	return GetLogger().WithError(err)
}

// Sync syncs the global logger
func Sync() error {
	return GetLogger().Sync()
}

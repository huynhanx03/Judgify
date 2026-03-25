package logger

import (
	"os"
	"path/filepath"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerZap wraps zap.Logger for structured logging.
type LoggerZap struct {
	*zap.Logger
}

// LoggerConfig holds configuration for logger initialization.
type LoggerConfig struct {
	Level      string
	Filename   string
	MaxSize    int // megabytes
	MaxBackups int
	MaxAge     int // days
	Compress   bool
}

// defaultConfig fills zero-valued fields with sensible production defaults.
func (c LoggerConfig) withDefaults() LoggerConfig {
	if c.Level == "" {
		c.Level = "info"
	}
	if c.Filename == "" {
		c.Filename = "./storages/logs/app.log"
	}
	if c.MaxSize == 0 {
		c.MaxSize = 100
	}
	if c.MaxBackups == 0 {
		c.MaxBackups = 5
	}
	if c.MaxAge == 0 {
		c.MaxAge = 30
	}
	return c
}

// NewLogger creates a production-ready logger with dual output:
//   - File: JSON format with rotation (for log aggregation)
//   - Console: colored human-readable format (for development)
func NewLogger(cfg LoggerConfig) *LoggerZap {
	cfg = cfg.withDefaults()

	// Ensure log directory exists based on configured filename
	logDir := filepath.Dir(cfg.Filename)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic("logger: failed to create log directory: " + err.Error())
	}

	logLevel := parseLevel(cfg.Level)

	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(fileEncoderConfig()),
		zapcore.AddSync(newRotator(cfg)),
		logLevel,
	)

	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(consoleEncoderConfig()),
		zapcore.AddSync(os.Stdout),
		logLevel,
	)

	core := zapcore.NewTee(fileCore, consoleCore)
	l := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &LoggerZap{Logger: l}
}

// fileEncoderConfig returns encoder config optimized for machine parsing.
func fileEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	return cfg
}

// consoleEncoderConfig returns encoder config optimized for dev readability.
func consoleEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return cfg
}

// newRotator creates a lumberjack rotator for log file management.
func newRotator(cfg LoggerConfig) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}
}

// parseLevel converts a string log level to zapcore.Level.
// Falls back to InfoLevel on unrecognized input.
func parseLevel(level string) zapcore.Level {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return zapcore.InfoLevel
	}
	return l
}

package infrastructure

import (
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/pkg/logger"

	"github.com/huynhanx03/judgify/global"
)

// SetupLogger initializes the logger and sets it as the global zap logger.
func SetupLogger() {
	config := logger.LoggerConfig{
		Level:      global.Config.Logger.LogLevel,
		Filename:   global.Config.Logger.FileLogName,
		MaxSize:    global.Config.Logger.MaxSize,
		MaxBackups: global.Config.Logger.MaxBackups,
		MaxAge:     global.Config.Logger.MaxAge,
		Compress:   global.Config.Logger.Compress,
	}

	global.LoggerZap = logger.NewLogger(config)

	// Set as global zap logger so logger.FromContext fallback works
	zap.ReplaceGlobals(global.LoggerZap.Logger)
}

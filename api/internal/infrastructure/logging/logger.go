// Package logging creates Judgify process loggers without acquiring any
// application resource. Keeping this boundary resource-free lets finite
// operator commands share the exact production logger policy without pulling
// API, worker, cache, or Ent graphs into their binaries.
package logging

import (
	"fmt"

	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/pkg/settings"
)

// New constructs the process root logger from validated configuration.
func New(config settings.Config) (*commonlogger.Logger, error) {
	initialized, err := commonlogger.NewLogger(commonlogger.LoggerConfig{
		Mode:       config.Server.Mode,
		Level:      config.Logger.LogLevel,
		Service:    "judgify",
		Version:    config.Observability.Version,
		Filename:   config.Logger.FileLogName,
		MaxSize:    config.Logger.MaxSize,
		MaxBackups: config.Logger.MaxBackups,
		MaxAge:     config.Logger.MaxAge,
		Compress:   config.Logger.Compress,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}
	// A root component keeps bootstrap and compatibility call sites compliant
	// with the same structured schema as explicitly named child loggers.
	return initialized.Named("application"), nil
}

package global

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/logger"
	"github.com/huynhanx03/judgify/pkg/settings"

	"github.com/huynhanx03/judgify/internal/ent"
)

var (
	Config    settings.Config
	LoggerZap *logger.LoggerZap
	EntClient *ent.EntClient
	Ember cache.LocalCache[string, any]
)

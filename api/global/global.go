package global

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/logger"
	"github.com/huynhanx03/judgify/pkg/settings"

	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
)

var (
	Config    settings.Config
	LoggerZap *logger.LoggerZap
	EntClient *dbEnt.EntClient
	Tinylfu   cache.LocalCache[string, any]
)

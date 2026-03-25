package infrastructure

import (
	"go.uber.org/zap"

	pkgEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	"github.com/huynhanx03/judgify/global"
	internalEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// SetupEnt initializes the Ent client for PostgreSQL.
func SetupEnt() {
	driver, err := pkgEnt.NewDriver(global.Config.Database)
	if err != nil {
		global.LoggerZap.Fatal("failed opening connection to ent", zap.Error(err))
	}

	client := generate.NewClient(generate.Driver(driver))

	global.LoggerZap.Named("db").Info("Database connected")
	global.EntClient = internalEnt.WrapClient(client, global.LoggerZap.Logger)
}

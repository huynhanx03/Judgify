package infrastructure

import (
	"log"

	pkgEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	"github.com/huynhanx03/judgify/global"
	internalEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// SetupEnt initializes the Ent client for PostgreSQL.
func SetupEnt() {
	driver, err := pkgEnt.NewDriver(global.Config.Database)
	if err != nil {
		log.Fatalf("failed opening connection to ent: %v", err)
	}

	client := generate.NewClient(generate.Driver(driver))

	global.EntClient = internalEnt.WrapClient(client, global.LoggerZap.Logger)
}

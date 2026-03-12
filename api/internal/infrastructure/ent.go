package infrastructure

import (
	"log"

	"github.com/huynhanx03/judgify/pkg/database/ent"

	"github.com/huynhanx03/judgify/global"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate"
)

// SetupEnt initializes the Ent client for PostgreSQL.
func SetupEnt() {
	driver, err := ent.NewDriver(global.Config.Database)
	if err != nil {
		log.Fatalf("failed opening connection to ent: %v", err)
	}

	client := generate.NewClient(generate.Driver(driver))

	global.EntClient = dbEnt.WrapClient(client, global.LoggerZap.Logger)
}

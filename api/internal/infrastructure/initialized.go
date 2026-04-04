package infrastructure

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/permissions"

	"github.com/huynhanx03/judgify/global"

	"go.uber.org/zap"
)

func Initialized() {
	setupResourceMapping()
}

func setupResourceMapping() {
	if global.EntClient == nil {
		global.LoggerZap.Fatal("Database client is not initialized")
	}

	resources, err := global.EntClient.DB(context.Background()).Resource.Query().All(context.Background())
	if err != nil {
		global.LoggerZap.Fatal("Failed to load resources for mapping", zap.Error(err))
	}

	mapping := make(map[string]int)
	for _, r := range resources {
		mapping[r.Key] = r.ID
	}

	permissions.SetResourceMap(mapping)
	global.LoggerZap.Info("Loaded resources into permission mapping", zap.Int("count", len(mapping)))
}

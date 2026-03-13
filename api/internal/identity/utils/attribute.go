package utils

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

// GetAttributeDefinition retrieves attribute definition by key with caching.
func GetAttributeDefinition(
	ctx context.Context,
	key string,
	repo ports.AttributeDefinitionRepository,
	localCache cache.LocalCache[string, any],
) (*entity.AttributeDefinition, error) {
	cacheKey := constant.CacheKeyPrefixAttrKey + key
	if d, found := cache.GetLocal[*entity.AttributeDefinition](localCache, cacheKey); found {
		return d, nil
	}

	def, err := repo.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	cache.SetLocal(localCache, cacheKey, def, constant.CacheCostAttrKey)
	return def, nil
}

// GetAttributeValue retrieves value of an attribute key from a list of user attributes.
func GetAttributeValue(
	ctx context.Context,
	key string,
	existingAttrs []*entity.UserAttributeValue,
	repo ports.AttributeDefinitionRepository,
	localCache cache.LocalCache[string, any],
) string {
	def, err := GetAttributeDefinition(ctx, key, repo, localCache)
	if err != nil {
		return ""
	}

	for _, attr := range existingAttrs {
		if attr.AttributeID == def.ID {
			return attr.Value
		}
	}
	return ""
}

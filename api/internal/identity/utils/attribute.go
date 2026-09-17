package utils

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

// GetAttributeDefinition retrieves attribute definition by key with caching.
func GetAttributeDefinition(
	ctx context.Context,
	key string,
	repo ports.AttributeDefinitionRepository,
	localCache cache.LocalCache[string, any],
) (*entity.AttributeDefinition, error) {
	cacheKey := constant.CacheKeyPrefixAttrKey + key
	if d, found := cache.Get[*entity.AttributeDefinition](localCache, cacheKey); found {
		return d, nil
	}

	def, err := repo.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	cache.SetWithTTL(localCache, cacheKey, def, constant.EntityCacheTTL)
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
		if attr != nil && attr.AttributeID == def.ID &&
			def.ActiveRevisionID != nil &&
			attr.DefinitionRevisionID == *def.ActiveRevisionID {
			return attr.Value
		}
	}
	return ""
}

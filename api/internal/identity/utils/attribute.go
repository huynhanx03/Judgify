package utils

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/cache"

	tenantcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

// GetAttributeDefinition retrieves attribute definition by key with caching.
func GetAttributeDefinition(
	ctx context.Context,
	key string,
	repo ports.AttributeDefinitionRepository,
	localCache *cache.Cache,
) (*entity.AttributeDefinition, error) {
	cacheKey := tenantcache.TenantScopedKey(ctx, constant.CacheKeyPrefixAttrKey+key)
	return cache.Fetch[*entity.AttributeDefinition](ctx, localCache, cacheKey, constant.EntityCacheTTL, func(loadCtx context.Context) (*entity.AttributeDefinition, error) {
		return repo.GetByKey(loadCtx, key)
	})
}

// GetAttributeValue retrieves value of an attribute key from a list of user attributes.
func GetAttributeValue(
	ctx context.Context,
	key string,
	existingAttrs []*entity.UserAttributeValue,
	repo ports.AttributeDefinitionRepository,
	localCache *cache.Cache,
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

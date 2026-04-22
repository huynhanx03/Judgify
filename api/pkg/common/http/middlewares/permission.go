package middlewares

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"
	"github.com/huynhanx03/judgify/pkg/permissions"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

// PermissionChecker holds dependencies for DB-based permission checking with local cache.
type PermissionChecker struct {
	userRepo       ports.UserRepository
	roleRepo       ports.RoleRepository
	permissionRepo ports.PermissionRepository
	cache          cache.LocalCache[string, any]
}

// NewPermissionChecker creates a new PermissionChecker instance.
func NewPermissionChecker(
	userRepo ports.UserRepository,
	roleRepo ports.RoleRepository,
	permissionRepo ports.PermissionRepository,
	localCache cache.LocalCache[string, any],
) *PermissionChecker {
	return &PermissionChecker{
		userRepo:       userRepo,
		roleRepo:       roleRepo,
		permissionRepo: permissionRepo,
		cache:          localCache,
	}
}

// getRolePermissions fetches aggregated permissions for a role (including descendants via Nested Set).
// Returns map[resourceID]scopeMask. Results are cached locally.
func (pc *PermissionChecker) getRolePermissions(ctx context.Context, roleID int) (map[int]int, error) {
	cacheKey := constant.CacheKeyPrefixRolePermissions + strconv.Itoa(roleID)
	if perms, found := cache.Get[map[int]int](pc.cache, cacheKey); found {
		return perms, nil
	}

	role, err := pc.roleRepo.Get(ctx, roleID)
	if err != nil {
		return nil, err
	}

	descendants, err := pc.roleRepo.FindDescendants(ctx, role.Lft, role.Rgt)
	if err != nil {
		return nil, err
	}

	roleIDs := make([]int, len(descendants))
	for i, r := range descendants {
		roleIDs[i] = r.ID
	}

	permEntities, err := pc.permissionRepo.FindByRoleIDs(ctx, roleIDs)
	if err != nil {
		return nil, err
	}

	perms := make(map[int]int)
	for _, p := range permEntities {
		perms[p.ResourceID] |= p.Scopes
	}

	cache.Set(pc.cache, cacheKey, perms)
	return perms, nil
}

// RequirePermission checks if the authenticated user's role has the required permission scope for a resource.
func (pc *PermissionChecker) RequirePermission(resourceKey string, requiredScope int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		userID, ok := ctx.Value(constraints.ContextKeyUserID).(int)
		if !ok {
			response.ErrorResponse(c, response.CodeUnauthorized, apperr.New(response.CodeUnauthorized, "user not authenticated", nil))
			c.Abort()
			return
		}

		user, err := pc.userRepo.Get(ctx, userID)
		if err != nil {
			response.ErrorResponse(c, response.CodeForbidden, apperr.New(response.CodeForbidden, "user not found", nil))
			c.Abort()
			return
		}

		perms, err := pc.getRolePermissions(ctx, user.RoleID)
		if err != nil {
			response.ErrorResponse(c, response.CodeForbidden, apperr.New(response.CodeForbidden, "failed to load permissions", nil))
			c.Abort()
			return
		}

		resourceID := permissions.GetResourceID(resourceKey)
		if resourceID == 0 {
			response.ErrorResponse(c, response.CodeForbidden, apperr.New(response.CodeForbidden, "unknown resource", nil))
			c.Abort()
			return
		}

		scopeMask, exists := perms[resourceID]
		if !exists || (scopeMask&requiredScope) != requiredScope {
			response.ErrorResponse(c, response.CodeForbidden, apperr.New(response.CodeForbidden, "permission denied", nil))
			c.Abort()
			return
		}

		c.Next()
	}
}

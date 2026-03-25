package constant

import "time"

const (
	// Cost
	CacheCostRolePermissions = 1
	CacheCostRoleName        = 1
	CacheCostAttrKey         = 1
	CacheCostID              = 1
	CacheCostName            = 1
	CacheCostKey             = 1

	// Keys
	CacheKeyPrefixAttrKey         = "attr_def::key::"
	CacheKeyPrefixRoleName        = "role::name::"
	CacheKeyPrefixRolePermissions = "role_perms::"
	CacheKeyPrefixRoleID          = "role::id::"
	CacheKeyPrefixAttrID          = "attr_def::id::"
	CacheKeyPrefixResourceID      = "resource::id::"
	CacheKeyPrefixResourceKey     = "resource::key::"
	CacheKeyPrefixPermID          = "perm::id::"
	// Permission Config Version
	CacheKeyPermissionConfigVersion = "perm_conf_ver"

	// Auth Cache Keys
	CacheKeyAuthBlacklistJTI    = "auth:blacklist:jti:"
	CacheKeyAuthRateLimitForgot = "auth:ratelimit:forgot:"

	// TTLs
	OAuthTokenTTL      = 10 * time.Minute
	ResetTokenTTL      = 15 * time.Minute
	ForgotRateLimitTTL = 1 * time.Minute
)

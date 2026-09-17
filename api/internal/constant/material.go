package constant

import "time"

const (
	// MaterialCacheTTL is the default time-to-live for read-through entries.
	MaterialCacheTTL = 10 * time.Minute

	MaterialCategoryByIDKeyPrefix = "judgify:v1:material:category:id:"
	MaterialCategoryAllKey        = "judgify:v1:material:category:all"

	MaterialStatusDraft     = "draft"
	MaterialStatusPublished = "published"
	MaterialStatusInReview  = "in_review"
	MaterialStatusArchived  = "archived"

	MaterialVisibilityPublic        = "public"
	MaterialVisibilityAuthenticated = "authenticated"
)

package http

import (
	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

// CultivationHandlerGroup holds all cultivation HTTP handlers.
type CultivationHandlerGroup struct {
	ElementHandler        ElementHandler
	TraitHandler          TraitHandler
	UserTraitHandler      UserTraitHandler
	UserElementExpHandler UserElementExpHandler
	LevelHandler          LevelHandler
	RankHandler           RankHandler
	UserStatsHandler      UserStatsHandler
}

// RegisterProtected registers all cultivation protected routes.
func (h *CultivationHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.PermissionChecker) {
	// Elements
	elements := r.Group("/elements", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeRead))
	{
		elements.POST("/find", handler.Wrap(h.ElementHandler.Find))
		elements.GET("/:id", handler.Wrap(h.ElementHandler.Get))
		elements.POST("", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeCreate), handler.Wrap(h.ElementHandler.Create))
		elements.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeUpdate), handler.Wrap(h.ElementHandler.Update))
		elements.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeDelete), handler.Wrap(h.ElementHandler.Delete))
	}

	// Traits
	traits := r.Group("/traits", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeRead))
	{
		traits.POST("/find", handler.Wrap(h.TraitHandler.Find))
		traits.GET("/:id", handler.Wrap(h.TraitHandler.Get))
		traits.POST("", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeCreate), handler.Wrap(h.TraitHandler.Create))
		traits.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeUpdate), handler.Wrap(h.TraitHandler.Update))
		traits.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeDelete), handler.Wrap(h.TraitHandler.Delete))
	}

	// User Traits
	userTraits := r.Group("/user-traits", permChecker.RequirePermission(permissions.ResourceKeyUserTrait, permissions.PermissionScopeRead))
	{
		userTraits.POST("/find", handler.Wrap(h.UserTraitHandler.Find))
		userTraits.GET("/:id", handler.Wrap(h.UserTraitHandler.Get))
		userTraits.POST("", permChecker.RequirePermission(permissions.ResourceKeyUserTrait, permissions.PermissionScopeCreate), handler.Wrap(h.UserTraitHandler.Create))
		userTraits.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyUserTrait, permissions.PermissionScopeDelete), handler.Wrap(h.UserTraitHandler.Delete))
	}

	// User Element EXP
	userElementExps := r.Group("/user-element-exps", permChecker.RequirePermission(permissions.ResourceKeyUserElementExp, permissions.PermissionScopeRead))
	{
		userElementExps.POST("/find", handler.Wrap(h.UserElementExpHandler.Find))
		userElementExps.GET("/:id", handler.Wrap(h.UserElementExpHandler.Get))
		userElementExps.POST("", permChecker.RequirePermission(permissions.ResourceKeyUserElementExp, permissions.PermissionScopeCreate), handler.Wrap(h.UserElementExpHandler.Create))
		userElementExps.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyUserElementExp, permissions.PermissionScopeUpdate), handler.Wrap(h.UserElementExpHandler.Update))
		userElementExps.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyUserElementExp, permissions.PermissionScopeDelete), handler.Wrap(h.UserElementExpHandler.Delete))
	}

	// Levels
	levels := r.Group("/levels", permChecker.RequirePermission(permissions.ResourceKeyLevel, permissions.PermissionScopeRead))
	{
		levels.POST("/find", handler.Wrap(h.LevelHandler.Find))
		levels.GET("/:id", handler.Wrap(h.LevelHandler.Get))
		levels.POST("", permChecker.RequirePermission(permissions.ResourceKeyLevel, permissions.PermissionScopeCreate), handler.Wrap(h.LevelHandler.Create))
		levels.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyLevel, permissions.PermissionScopeUpdate), handler.Wrap(h.LevelHandler.Update))
		levels.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyLevel, permissions.PermissionScopeDelete), handler.Wrap(h.LevelHandler.Delete))
	}

	// Ranks
	ranks := r.Group("/ranks", permChecker.RequirePermission(permissions.ResourceKeyRank, permissions.PermissionScopeRead))
	{
		ranks.POST("/find", handler.Wrap(h.RankHandler.Find))
		ranks.GET("/:id", handler.Wrap(h.RankHandler.Get))
		ranks.POST("", permChecker.RequirePermission(permissions.ResourceKeyRank, permissions.PermissionScopeCreate), handler.Wrap(h.RankHandler.Create))
		ranks.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyRank, permissions.PermissionScopeUpdate), handler.Wrap(h.RankHandler.Update))
		ranks.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyRank, permissions.PermissionScopeDelete), handler.Wrap(h.RankHandler.Delete))
	}

	// User Stats
	userStats := r.Group("/user-stats", permChecker.RequirePermission(permissions.ResourceKeyUserStats, permissions.PermissionScopeRead))
	{
		userStats.POST("/find", handler.Wrap(h.UserStatsHandler.Find))
		userStats.GET("/:id", handler.Wrap(h.UserStatsHandler.Get))
		userStats.POST("", permChecker.RequirePermission(permissions.ResourceKeyUserStats, permissions.PermissionScopeCreate), handler.Wrap(h.UserStatsHandler.Create))
		userStats.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyUserStats, permissions.PermissionScopeUpdate), handler.Wrap(h.UserStatsHandler.Update))
		userStats.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyUserStats, permissions.PermissionScopeDelete), handler.Wrap(h.UserStatsHandler.Delete))
	}
}

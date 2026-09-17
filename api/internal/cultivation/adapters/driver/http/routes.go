package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

// CultivationHandlerGroup holds all cultivation HTTP handlers.
type CultivationHandlerGroup struct {
	ElementHandler               ElementHandler
	TraitHandler                 TraitHandler
	UserTraitHandler             UserTraitHandler
	LevelHandler                 LevelHandler
	RankHandler                  RankHandler
	RarityHandler                RarityHandler
	RankingHandler               RankingHandler
	ProgressionHandler           ProgressionHandler
	RewardRuleHandler            RewardRuleHandler
	RewardProfileHandler         RewardProfileHandler
	RankingReconciliationHandler RankingReconciliationHandler
}

// RegisterPublic registers public cultivation routes (no auth required).
func (h *CultivationHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
	traits := r.Group("/traits")
	{
		traits.GET("", handler.WrapNoReq(h.TraitHandler.FindAll))
		traits.POST("/find", handler.Wrap(h.TraitHandler.Find))
		traits.GET("/:id", handler.Wrap(h.TraitHandler.Get))
	}

	elements := r.Group("/elements")
	{
		elements.GET("", handler.WrapNoReq(h.ElementHandler.FindAll))
	}

	levels := r.Group("/levels")
	{
		levels.GET("", handler.WrapNoReq(h.LevelHandler.FindAll))
	}

	ranks := r.Group("/ranks")
	{
		ranks.GET("", handler.WrapNoReq(h.RankHandler.FindAll))
	}

	rarities := r.Group("/rarities")
	{
		rarities.GET("", handler.WrapNoReq(h.RarityHandler.FindAll))
	}

	rankings := r.Group("/rankings")
	{
		rankings.GET("/rating", handler.Wrap(h.RankingHandler.GetByRating))
		rankings.GET("/exp", handler.Wrap(h.RankingHandler.GetByExperience))
	}
}

// RegisterProtected registers all cultivation protected routes.
func (h *CultivationHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.AuthorizationChecker) {
	rewardProfile := r.Group("/reward-profile")
	{
		rewardProfile.GET("", cultivationNoStore, handler.WrapNoReq(h.RewardProfileHandler.CurrentSelf))
	}
	progression := r.Group("/progression")
	{
		progression.GET("/ledger", cultivationNoStore, handler.Wrap(h.ProgressionHandler.ListMyLedger))
	}
	rankingPrivacy := r.Group("/ranking/privacy")
	{
		rankingPrivacy.GET("", cultivationNoStore, handler.WrapNoReq(h.RankingHandler.Privacy))
		rankingPrivacy.PUT("", cultivationNoStore, captureCultivationCommandID(), handler.Wrap(h.RankingHandler.UpdatePrivacy))
	}

	adminCultivation := r.Group("/admin/cultivation")
	adminCultivation.Use(cultivationNoStore)
	{
		adminCultivation.GET(
			"/progression/ledger",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeRead),
			handler.Wrap(h.ProgressionHandler.ListLedger),
		)
		adminCultivation.POST(
			"/ranking/reconciliation/preview",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeRebuild),
			h.RankingReconciliationHandler.Preview,
		)
		adminCultivation.POST(
			"/progression/adjustments/preview",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeAdjust),
			handler.Wrap(h.ProgressionHandler.PreviewAdjustment),
		)
		adminCultivation.POST(
			"/progression/adjustments",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeAdjust),
			captureCultivationCommandID(), handler.Wrap(h.ProgressionHandler.ApplyAdjustment),
		)
		adminCultivation.POST(
			"/progression/reversals/preview",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeReverse),
			handler.Wrap(h.ProgressionHandler.PreviewAdjustment),
		)
		adminCultivation.POST(
			"/progression/reversals",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeReverse),
			captureCultivationCommandID(), handler.Wrap(h.ProgressionHandler.ApplyAdjustment),
		)
		adminCultivation.GET(
			"/reward-rules",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationReward, permissions.PermissionScopeRead),
			handler.Wrap(h.RewardRuleHandler.List),
		)
		adminCultivation.POST(
			"/reward-rules/preview",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationReward, permissions.PermissionScopePublish),
			handler.Wrap(h.RewardRuleHandler.PreviewPublication),
		)
		adminCultivation.POST(
			"/reward-rules",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationReward, permissions.PermissionScopePublish),
			captureCultivationCommandID(), handler.Wrap(h.RewardRuleHandler.Publish),
		)
		adminCultivation.GET(
			"/reward-profiles/:user_id",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationReward, permissions.PermissionScopeRead),
			handler.Wrap(h.RewardProfileHandler.Current),
		)
		adminCultivation.POST(
			"/reward-profiles/:user_id/preview",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationReward, permissions.PermissionScopePublish),
			handler.Wrap(h.RewardProfileHandler.PreviewAssignment),
		)
		adminCultivation.POST(
			"/reward-profiles/:user_id",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationReward, permissions.PermissionScopePublish),
			captureCultivationCommandID(), handler.Wrap(h.RewardProfileHandler.Assign),
		)
		adminCultivation.GET(
			"/ranking/reconciliation",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeRead),
			handler.WrapNoReq(h.RankingReconciliationHandler.Status),
		)
		adminCultivation.POST(
			"/ranking/reconciliation",
			permChecker.RequirePermission(permissions.ResourceKeyCultivationLedger, permissions.PermissionScopeRebuild),
			captureCultivationCommandID(), h.RankingReconciliationHandler.Start,
		)
	}

	// Elements
	elements := r.Group("/elements", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeRead))
	{
		elements.POST("/find", handler.Wrap(h.ElementHandler.Find))
		elements.GET("/:id", handler.Wrap(h.ElementHandler.Get))
		elements.POST("", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeCreate), handler.Wrap(h.ElementHandler.Create))
		elements.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeUpdate), handler.Wrap(h.ElementHandler.Update))
		elements.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyElement, permissions.PermissionScopeDelete), handler.Wrap(h.ElementHandler.Delete))
	}

	// Trait discovery is public; catalog mutations remain protected.
	traits := r.Group("/traits", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeRead))
	{
		traits.POST("", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeCreate), handler.Wrap(h.TraitHandler.Create))
		traits.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeUpdate), handler.Wrap(h.TraitHandler.Update))
		traits.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyTrait, permissions.PermissionScopeDelete), handler.Wrap(h.TraitHandler.Delete))
	}

	// User Traits
	userTraits := r.Group("/user-traits", permChecker.RequirePermission(permissions.ResourceKeyUserTrait, permissions.PermissionScopeRead))
	{
		userTraits.POST("/find", handler.Wrap(h.UserTraitHandler.Find))
		userTraits.GET("/:id", handler.Wrap(h.UserTraitHandler.Get))
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

	// Rarities
	rarities := r.Group("/rarities", permChecker.RequirePermission(permissions.ResourceKeyRarity, permissions.PermissionScopeRead))
	{
		rarities.POST("/find", handler.Wrap(h.RarityHandler.Find))
		rarities.GET("/:id", handler.Wrap(h.RarityHandler.Get))
		rarities.POST("", permChecker.RequirePermission(permissions.ResourceKeyRarity, permissions.PermissionScopeCreate), handler.Wrap(h.RarityHandler.Create))
		rarities.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyRarity, permissions.PermissionScopeUpdate), handler.Wrap(h.RarityHandler.Update))
		rarities.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyRarity, permissions.PermissionScopeDelete), handler.Wrap(h.RarityHandler.Delete))
	}

}

func cultivationNoStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Next()
}

func captureCultivationCommandID() gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Idempotency-Key")
		if len(values) == 1 {
			commandID, err := uuid.Parse(values[0])
			if err == nil && commandID != uuid.Nil && commandID.String() == values[0] {
				c.Request = c.Request.WithContext(context.WithValue(
					c.Request.Context(), cultivationCommandIDContextKey{}, commandID,
				))
			}
		}
		c.Next()
	}
}

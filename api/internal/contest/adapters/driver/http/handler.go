package http

import (
	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

// ContestHandlerGroup contains all contest-related handlers.
type ContestHandlerGroup struct {
	ContestHandler      ContestHandler
	RegistrationHandler RegistrationHandler
	StandingHandler     *standingHandler
	RatingHandler       RatingHandler
}

// RegisterPublic registers public contest routes.
func (h *ContestHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
	contests := r.Group("/contests")
	{
		contests.POST("/find", handler.Wrap(h.ContestHandler.Find))
		contests.GET("/:id", handler.Wrap(h.ContestHandler.Get))
		contests.GET("/:id/standings", handler.Wrap(h.StandingHandler.GetStandings))
		contests.GET("/:id/standings/stream", h.StandingHandler.StreamStandings)
		contests.GET("/:id/rating-changes", handler.Wrap(h.RatingHandler.GetContestRatingChanges))
	}
}

// RegisterProtected registers protected contest routes.
func (h *ContestHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.PermissionChecker) {
	contests := r.Group("/contests")
	{
		// Contest CRUD (admin)
		contests.POST("", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeCreate), handler.Wrap(h.ContestHandler.Create))
		contests.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), handler.Wrap(h.ContestHandler.Update))
		contests.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeDelete), handler.Wrap(h.ContestHandler.Delete))

		// Registration (authenticated users)
		contests.POST("/:id/register", handler.Wrap(h.RegistrationHandler.Register))
		contests.POST("/:id/unregister", handler.Wrap(h.RegistrationHandler.Unregister))
	}
}

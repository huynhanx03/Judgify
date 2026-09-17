package http

import (
	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

// ContestHandlerGroup contains all contest-related handlers.
type ContestHandlerGroup struct {
	ContestHandler      ContestHandler
	RegistrationHandler RegistrationHandler
	StandingHandler     *standingHandler
	RatingHandler       RatingHandler
	ContentHandler      ContentHandler
}

// RegisterPublic registers public contest routes.
func (h *ContestHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
	contests := r.Group("/contests")
	{
		contests.POST("/find", handler.Wrap(h.ContestHandler.FindPublic))
		contests.GET("/:id", handler.Wrap(h.ContestHandler.GetPublic))
		contests.GET(
			"/:id/problems/:contest_problem_id",
			handler.Wrap(h.ContestHandler.GetProblem),
		)
		contests.GET("/:id/standings", handler.Wrap(h.StandingHandler.GetStandings))
		contests.GET("/:id/rating-changes", handler.Wrap(h.RatingHandler.GetContestRatingChanges))
		contests.GET("/:id/announcements", handler.Wrap(h.ContentHandler.ListPublicAnnouncements))
	}
}

// RegisterProtected registers protected contest routes.
func (h *ContestHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.AuthorizationChecker) {
	adminContestQueries := r.Group("/admin/contests")
	{
		adminContestQueries.POST("/find", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeRead), handler.Wrap(h.ContestHandler.FindAdmin))
		adminContestQueries.GET("/:id", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeRead), handler.Wrap(h.ContestHandler.GetAdmin))
	}

	contests := r.Group("/contests")
	{
		contests.GET(
			"/:id/standings/me",
			contestContentNoStore,
			handler.Wrap(h.StandingHandler.GetMyStanding),
		)
		contests.GET("/:id/announcements/participants", contestContentNoStore, handler.Wrap(h.ContentHandler.ListParticipantAnnouncements))
		contests.GET("/:id/clarifications", contestContentNoStore, handler.Wrap(h.ContentHandler.ListParticipantClarifications))
		contests.GET("/:id/clarifications/mine", contestContentNoStore, handler.Wrap(h.ContentHandler.ListOwnClarifications))
		contests.GET("/:id/clarifications/mine/:content_id", contestContentNoStore, handler.Wrap(h.ContentHandler.GetOwnClarification))
		contests.POST("/:id/clarifications", contestContentNoStore, h.ContentHandler.SubmitClarification)
		contests.POST("/:id/clarifications/:content_id/withdraw", contestContentNoStore, h.ContentHandler.WithdrawClarification)
		contests.POST("/:id/announcements", contestContentNoStore, permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.CreateAnnouncement)
		contests.PUT("/:id/announcements/:content_id", contestContentNoStore, permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.EditAnnouncement)
		contests.POST("/:id/announcements/:content_id/publish", contestContentNoStore, permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.PublishAnnouncement)
		contests.POST("/:id/announcements/:content_id/withdraw", contestContentNoStore, permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.WithdrawAnnouncement)
		contests.POST("/:id/clarifications/:content_id/answer", contestContentNoStore, permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.AnswerClarification)
		contests.POST("/:id/clarifications/:content_id/close", contestContentNoStore, permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.CloseClarification)
		// Contest CRUD (admin)
		contests.POST("", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeCreate), handler.Wrap(h.ContestHandler.Create))
		contests.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), handler.Wrap(h.ContestHandler.Update))
		contests.POST(
			"/:id/publish",
			permChecker.RequirePermission(
				permissions.ResourceKeyContest,
				permissions.PermissionScopeUpdate,
			),
			h.ContestHandler.PublishAccepted,
		)
		contests.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeDelete), handler.Wrap(h.ContestHandler.Delete))

		// Registration (authenticated users)
		contests.POST(
			"/:id/register",
			h.RegistrationHandler.RegisterAccepted,
		)
		contests.POST(
			"/:id/unregister",
			h.RegistrationHandler.WithdrawAccepted,
		)
	}
	adminContests := r.Group("/admin/contests", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeRead))
	adminContests.Use(contestContentNoStore)
	{
		adminContests.GET("/:id/standings", handler.Wrap(h.StandingHandler.GetOfficialStandings))
		adminContests.POST(
			"/:id/end",
			permChecker.RequirePermission(
				permissions.ResourceKeyContest,
				permissions.PermissionScopeUpdate,
			),
			h.ContestHandler.EndAccepted,
		)
		adminContests.POST(
			"/:id/cancel",
			permChecker.RequirePermission(
				permissions.ResourceKeyContest,
				permissions.PermissionScopeUpdate,
			),
			h.ContestHandler.CancelAccepted,
		)
		adminContests.POST(
			"/:id/rating/rerate",
			permChecker.RequirePermission(
				permissions.ResourceKeyContest,
				permissions.PermissionScopeUpdate,
			),
			permChecker.RequirePermission(
				permissions.ResourceKeyOperation,
				permissions.PermissionScopeExecute,
			),
			h.RatingHandler.StartRerating,
		)
		adminContests.GET("/:id/announcements", handler.Wrap(h.ContentHandler.ListAdminAnnouncements))
		adminContests.GET("/:id/announcements/:content_id", handler.Wrap(h.ContentHandler.GetAdminAnnouncement))
		adminContests.GET("/:id/announcements/:content_id/revisions", handler.Wrap(h.ContentHandler.ListAnnouncementRevisions))
		adminContests.GET("/:id/clarifications", handler.Wrap(h.ContentHandler.ListAdminClarifications))
		adminContests.GET("/:id/clarifications/:content_id", handler.Wrap(h.ContentHandler.GetAdminClarification))
		adminContests.GET("/:id/clarifications/:content_id/revisions", handler.Wrap(h.ContentHandler.ListClarificationRevisions))
		adminContests.POST("/:id/clarifications/:content_id/answer", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.AnswerClarification)
		adminContests.POST("/:id/clarifications/:content_id/close", permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeUpdate), h.ContentHandler.CloseClarification)
	}
}

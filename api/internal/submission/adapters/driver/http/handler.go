package http

import (
	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
)

type SubmissionHandlerGroup struct {
	SubmissionHandler SubmissionHandler
}

func (h *SubmissionHandlerGroup) RegisterProtected(r *gin.RouterGroup) {
	submissions := r.Group("/submissions")
	{
		submissions.POST("", handler.Wrap(h.SubmissionHandler.Submit))
		submissions.GET("/:id", handler.Wrap(h.SubmissionHandler.Get))
	}

	// List submissions by problem (nested under problems)
	r.GET("/problems/:id/submissions", handler.Wrap(h.SubmissionHandler.FindByProblem))
	r.GET("/problems/:id/my-submissions", handler.Wrap(h.SubmissionHandler.FindMySubmissions))
}

package http

import (
	"context"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	handlerCommon "github.com/huynhanx03/judgify/pkg/common/http/handler"
)

// RatingHandler defines the rating HTTP handler interface.
type RatingHandler interface {
	GetContestRatingChanges(ctx context.Context, req *dto.GetContestRatingChangesRequest) ([]*dto.RatingChangeResponse, error)
}

type ratingHandler struct {
	handlerCommon.BaseHandler
	ratingService ports.RatingService
}

// NewRatingHandler creates a new RatingHandler instance.
func NewRatingHandler(ratingService ports.RatingService) RatingHandler {
	return &ratingHandler{ratingService: ratingService}
}

func (h *ratingHandler) GetContestRatingChanges(ctx context.Context, req *dto.GetContestRatingChangesRequest) ([]*dto.RatingChangeResponse, error) {
	return h.ratingService.GetContestRatingChanges(ctx, req.ContestID)
}

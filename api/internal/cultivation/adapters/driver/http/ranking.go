package http

import (
	"context"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type RankingHandler interface {
	GetByRating(context.Context, *rankingPageRequest) (*rankingPageResponse, error)
	GetByExperience(context.Context, *rankingPageRequest) (*rankingPageResponse, error)
	Privacy(context.Context) (*rankingPrivacyResponse, error)
	UpdatePrivacy(context.Context, *updateRankingPrivacyRequest) (*rankingPrivacyResponse, error)
}

func (handler *rankingHandler) Privacy(ctx context.Context) (*rankingPrivacyResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok || userID == uuid.Nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	response, err := handler.rankingService.Privacy(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toRankingPrivacyResponse(response), nil
}

func (handler *rankingHandler) UpdatePrivacy(
	ctx context.Context,
	request *updateRankingPrivacyRequest,
) (*rankingPrivacyResponse, error) {
	userID, ok := authctx.UserID(ctx)
	commandID, commandOK := ctx.Value(cultivationCommandIDContextKey{}).(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	if !commandOK || commandID == uuid.Nil {
		return nil, invalidCultivationCommand()
	}
	response, err := handler.rankingService.UpdatePrivacy(ctx, entity.RankingPrivacyCommand{
		ID: commandID, UserID: userID,
		Visibility:      entity.RankingVisibility(request.Visibility),
		ExpectedVersion: request.ExpectedVersion, Reason: request.Reason,
	})
	if err != nil {
		return nil, err
	}
	return toRankingPrivacyResponse(response), nil
}

type rankingHandler struct {
	rankingService ports.RankingService
}

func NewRankingHandler(rankingService ports.RankingService) RankingHandler {
	return &rankingHandler{rankingService: rankingService}
}

func (handler *rankingHandler) GetByRating(
	ctx context.Context,
	request *rankingPageRequest,
) (*rankingPageResponse, error) {
	response, err := handler.rankingService.Page(ctx, entity.RankingByRating, request.Cursor, request.Limit)
	if err != nil {
		return nil, err
	}
	return toRankingPageResponse(response), nil
}

func (handler *rankingHandler) GetByExperience(
	ctx context.Context,
	request *rankingPageRequest,
) (*rankingPageResponse, error) {
	response, err := handler.rankingService.Page(ctx, entity.RankingByExperience, request.Cursor, request.Limit)
	if err != nil {
		return nil, err
	}
	return toRankingPageResponse(response), nil
}

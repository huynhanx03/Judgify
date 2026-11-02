package http

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	handlerCommon "github.com/huynhanx03/go-common/pkg/common/http/handler"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
)

// StandingHandler defines the standing HTTP handler interface.
type StandingHandler interface {
	GetStandings(ctx context.Context, req *standingContestIDRequest) (*standingsResponse, error)
	GetMyStanding(ctx context.Context, req *standingContestIDRequest) (*selfStandingResponse, error)
	GetOfficialStandings(ctx context.Context, req *standingContestIDRequest) (*standingsResponse, error)
}

func (h *standingHandler) GetMyStanding(
	ctx context.Context,
	req *standingContestIDRequest,
) (*selfStandingResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", nil)
	}
	id, err := parseStandingContestID(req)
	if err != nil {
		return nil, err
	}
	response, err := h.standingService.GetMyStanding(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return toSelfStandingResponse(response), nil
}

type standingHandler struct {
	handlerCommon.BaseHandler
	standingService ports.StandingService
}

// NewStandingHandler creates a new standingHandler instance.
func NewStandingHandler(standingService ports.StandingService) *standingHandler {
	return &standingHandler{standingService: standingService}
}

func (h *standingHandler) GetStandings(ctx context.Context, req *standingContestIDRequest) (*standingsResponse, error) {
	id, err := parseStandingContestID(req)
	if err != nil {
		return nil, err
	}
	response, err := h.standingService.GetStandings(ctx, id)
	if err != nil {
		return nil, err
	}
	return toStandingsResponse(response), nil
}

func (h *standingHandler) GetOfficialStandings(ctx context.Context, req *standingContestIDRequest) (*standingsResponse, error) {
	id, err := parseStandingContestID(req)
	if err != nil {
		return nil, err
	}
	response, err := h.standingService.GetOfficialStandings(ctx, id)
	if err != nil {
		return nil, err
	}
	return toStandingsResponse(response), nil
}

func parseStandingContestID(request *standingContestIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ContestID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse contest id: %w", err)
	}
	return id, nil
}

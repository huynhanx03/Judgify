package http

import (
	"context"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	handlerCommon "github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"
)

// RegistrationHandler defines the registration HTTP handler interface.
type RegistrationHandler interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (any, error)
	Unregister(ctx context.Context, req *dto.UnregisterRequest) (any, error)
}

type registrationHandler struct {
	handlerCommon.BaseHandler
	regService ports.RegistrationService
}

// NewRegistrationHandler creates a new RegistrationHandler instance.
func NewRegistrationHandler(regService ports.RegistrationService) RegistrationHandler {
	return &registrationHandler{regService: regService}
}

func (h *registrationHandler) Register(ctx context.Context, req *dto.RegisterRequest) (any, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeUnauthorized, "unauthorized", nil)
	}
	if err := h.regService.Register(ctx, req.ContestID, userID); err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *registrationHandler) Unregister(ctx context.Context, req *dto.UnregisterRequest) (any, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeUnauthorized, "unauthorized", nil)
	}
	if err := h.regService.Unregister(ctx, req.ContestID, userID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ensure handler.Wrap is available
var _ = handler.Wrap[any, any]

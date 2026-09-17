package http

import (
	"context"

	"github.com/huynhanx03/judgify/internal/ports"
)

type ProfileSchemaHandler interface {
	GetOnboardingSchema(context.Context) (*profileSchemaResponse, error)
}

type profileSchemaHandler struct{ service ports.ProfileSchemaService }

func NewProfileSchemaHandler(service ports.ProfileSchemaService) ProfileSchemaHandler {
	return &profileSchemaHandler{service: service}
}

func (handler *profileSchemaHandler) GetOnboardingSchema(
	ctx context.Context,
) (*profileSchemaResponse, error) {
	result, err := handler.service.GetOnboardingSchema(ctx)
	if err != nil {
		return nil, err
	}
	return toProfileSchemaResponse(result), nil
}

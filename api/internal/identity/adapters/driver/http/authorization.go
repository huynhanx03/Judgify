package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
)

// AuthorizationHandler exposes only the code-owned capability catalog. Policy
// writes remain attached to their role/user resources and revision fences.
type AuthorizationHandler interface {
	Catalog(context.Context) (*authorizationCatalogResponse, error)
	Me(context.Context) (*effectiveCapabilitiesResponse, error)
}

type authorizationQueryReader interface {
	ports.AuthorizationCatalogReader
	ports.EffectiveCapabilityReader
}

type authorizationHandler struct {
	reader authorizationQueryReader
}

func NewAuthorizationHandler(reader authorizationQueryReader) AuthorizationHandler {
	return &authorizationHandler{reader: reader}
}

func (handler *authorizationHandler) Catalog(ctx context.Context) (*authorizationCatalogResponse, error) {
	result, err := handler.reader.Catalog(ctx)
	return toAuthorizationCatalogResponse(result), err
}

func (handler *authorizationHandler) Me(ctx context.Context) (*effectiveCapabilitiesResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", nil)
	}
	result, err := handler.reader.EffectiveCapabilities(ctx, userID)
	return toEffectiveCapabilitiesResponse(result), err
}

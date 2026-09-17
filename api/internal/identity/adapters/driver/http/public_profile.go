package http

import (
	"context"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type PublicProfileHandler interface {
	GetPublic(
		ctx context.Context,
		req *getPublicProfileRequest,
	) (*publicProfileResponse, error)
	GetPrivacy(context.Context) (*profilePrivacyResponse, error)
	UpdatePrivacy(
		ctx context.Context,
		req *updateProfilePrivacyRequest,
	) (*profilePrivacyMutationReceipt, error)
}

type publicProfileHandler struct {
	service ports.PublicProfileService
}

func NewPublicProfileHandler(
	service ports.PublicProfileService,
) (PublicProfileHandler, error) {
	if service == nil {
		return nil, ports.ErrInvalidPublicProfile
	}
	return &publicProfileHandler{service: service}, nil
}

func (handler *publicProfileHandler) GetPublic(
	ctx context.Context,
	req *getPublicProfileRequest,
) (*publicProfileResponse, error) {
	if handler == nil || handler.service == nil || req == nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid public profile request",
			ports.ErrInvalidPublicProfile,
		)
	}
	result, err := handler.service.GetPublic(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	return toPublicProfileResponse(result), nil
}

func (handler *publicProfileHandler) GetPrivacy(ctx context.Context) (*profilePrivacyResponse, error) {
	if handler == nil || handler.service == nil {
		return nil, invalidProfilePrivacyRequest()
	}
	userID, ok := authctx.UserID(ctx)
	if !ok || userID == uuid.Nil {
		return nil, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	result, err := handler.service.GetPrivacy(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toProfilePrivacyResponse(result), nil
}

func (handler *publicProfileHandler) UpdatePrivacy(
	ctx context.Context,
	req *updateProfilePrivacyRequest,
) (*profilePrivacyMutationReceipt, error) {
	if handler == nil || handler.service == nil || req == nil {
		return nil, invalidProfilePrivacyRequest()
	}
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	commandID, commandOK := identityCommandID(ctx)
	if !userOK || !sessionOK || userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !commandOK || commandID == uuid.Nil {
		return nil, invalidProfilePrivacyRequest()
	}
	result, err := handler.service.UpdatePrivacy(ctx, ports.UpdateProfilePrivacyCommand{
		UserID:          userID,
		ActorID:         userID,
		ActorSessionID:  sessionID,
		CommandID:       commandID,
		Visibility:      entity.ProfileVisibility(req.Visibility),
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return toProfilePrivacyMutationReceipt(result), nil
}

func invalidProfilePrivacyRequest() error {
	return apperr.New(
		apperr.CodeBadRequest,
		"invalid profile privacy request",
		ports.ErrInvalidProfilePrivacyCommand,
	)
}

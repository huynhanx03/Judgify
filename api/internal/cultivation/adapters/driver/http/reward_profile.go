package http

import (
	"context"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type RewardProfileHandler interface {
	CurrentSelf(context.Context) (*rewardProfileResponse, error)
	Current(context.Context, *rewardProfileUserRequest) (*rewardProfileResponse, error)
	PreviewAssignment(context.Context, *previewRewardProfileAssignmentRequest) (*rewardProfileAssignmentPreviewResponse, error)
	Assign(context.Context, *assignRewardProfileRequest) (*rewardProfileResponse, error)
}

type rewardProfileHandler struct {
	service ports.RewardProfileService
}

func NewRewardProfileHandler(service ports.RewardProfileService) RewardProfileHandler {
	return &rewardProfileHandler{service: service}
}

func (handler *rewardProfileHandler) CurrentSelf(
	ctx context.Context,
) (*rewardProfileResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok || userID == uuid.Nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	profile, err := handler.service.Current(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toRewardProfileResponse(profile, false), nil
}

func (handler *rewardProfileHandler) Current(
	ctx context.Context,
	request *rewardProfileUserRequest,
) (*rewardProfileResponse, error) {
	userID, err := toRewardProfileUserID(request.UserID)
	if err != nil {
		return nil, err
	}
	profile, err := handler.service.Current(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toRewardProfileResponse(profile, false), nil
}

func (handler *rewardProfileHandler) PreviewAssignment(
	ctx context.Context,
	request *previewRewardProfileAssignmentRequest,
) (*rewardProfileAssignmentPreviewResponse, error) {
	userID, err := toRewardProfileUserID(request.UserID)
	if err != nil {
		return nil, err
	}
	selection, err := toRewardProfileSelection(request.rewardProfileSelectionRequest)
	if err != nil {
		return nil, err
	}
	preview, err := handler.service.PreviewAssignment(
		ctx,
		userID,
		selection,
	)
	if err != nil {
		return nil, err
	}
	return &rewardProfileAssignmentPreviewResponse{
		UserID:          preview.UserID.String(),
		CurrentRevision: preview.CurrentRevision,
		NextRevision:    preview.NextRevision,
		Traits:          toRewardProfileTraitResponses(preview.Traits),
		PreviewChecksum: hex.EncodeToString(preview.Checksum[:]),
	}, nil
}

func (handler *rewardProfileHandler) Assign(
	ctx context.Context,
	request *assignRewardProfileRequest,
) (*rewardProfileResponse, error) {
	userID, err := toRewardProfileUserID(request.UserID)
	if err != nil {
		return nil, err
	}
	selection, err := toRewardProfileSelection(request.rewardProfileSelectionRequest)
	if err != nil {
		return nil, err
	}
	commandID, commandOK := ctx.Value(cultivationCommandIDContextKey{}).(uuid.UUID)
	previewChecksum, checksumOK := decodeCanonicalChecksum(request.PreviewChecksum)
	if !commandOK || commandID == uuid.Nil || !checksumOK {
		return nil, invalidCultivationCommand()
	}
	result, err := handler.service.Assign(ctx, entity.RewardProfileAssignmentCommand{
		ID:               commandID,
		UserID:           userID,
		ExpectedRevision: request.ExpectedRevision,
		Selection:        selection,
		PreviewChecksum:  previewChecksum,
		Reason:           request.Reason,
	})
	if err != nil {
		return nil, err
	}
	return toRewardProfileResponse(result.Profile, result.Replayed), nil
}

func toRewardProfileResponse(
	profile entity.UserRewardProfileRevision,
	replayed bool,
) *rewardProfileResponse {
	return &rewardProfileResponse{
		ID:                 profile.ID.String(),
		UserID:             profile.UserID.String(),
		Revision:           profile.Revision,
		AssignmentChecksum: hex.EncodeToString(profile.AssignmentChecksum[:]),
		Source:             profile.Source,
		SourceOfferID:      stringProgressionUUID(profile.SourceOfferID),
		CommandID:          stringProgressionUUID(profile.CommandID),
		EffectiveAt:        profile.EffectiveAt,
		Traits:             toRewardProfileTraitResponses(profile.Traits),
		Replayed:           replayed,
	}
}

func toRewardProfileTraitResponses(
	traits []entity.RewardProfileTrait,
) []rewardProfileTraitResponse {
	responses := make([]rewardProfileTraitResponse, len(traits))
	for index, assignment := range traits {
		responses[index] = rewardProfileTraitResponse{
			TraitID:     assignment.TraitID.String(),
			TraitCode:   assignment.TraitCode,
			TraitType:   assignment.TraitType,
			TraitName:   assignment.TraitName,
			Description: assignment.Description,
			RarityCode:  assignment.RarityCode,
			RarityName:  assignment.RarityName,
			Slot:        assignment.Slot,
			SlotOrder:   assignment.SlotOrder,
			EffectRevision: traitEffectRevisionResponse{
				ID:          assignment.EffectRevision.ID.String(),
				Revision:    assignment.EffectRevision.Revision,
				Effect:      assignment.EffectRevision.Effect,
				Checksum:    hex.EncodeToString(assignment.EffectRevision.Checksum[:]),
				EffectiveAt: assignment.EffectRevision.EffectiveAt,
			},
		}
	}
	return responses
}

package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type rewardProfileService struct {
	repository ports.RewardProfileRepository
	journal    platformmutation.Executor
}

func NewRewardProfileService(
	repository ports.RewardProfileRepository,
	journal platformmutation.Executor,
) (ports.RewardProfileService, error) {
	if repository == nil || journal == nil {
		return nil, entity.ErrInvalidRewardProfile
	}
	return &rewardProfileService{repository: repository, journal: journal}, nil
}

func (service *rewardProfileService) Current(
	ctx context.Context,
	userID uuid.UUID,
) (entity.UserRewardProfileRevision, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return entity.UserRewardProfileRevision{}, invalidRewardProfileRequest(
			entity.ErrInvalidRewardProfile,
		)
	}
	profile, err := service.repository.Current(ctx, userID)
	return profile, mapRewardProfileError(err)
}

func (service *rewardProfileService) PreviewAssignment(
	ctx context.Context,
	userID uuid.UUID,
	selection entity.RewardProfileSelection,
) (entity.RewardProfileAssignmentPreview, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return entity.RewardProfileAssignmentPreview{}, invalidRewardProfileRequest(
			entity.ErrInvalidRewardProfile,
		)
	}
	preview, err := service.repository.PreviewAssignment(ctx, userID, selection)
	return preview, mapRewardProfileError(err)
}

func (service *rewardProfileService) Assign(
	ctx context.Context,
	command entity.RewardProfileAssignmentCommand,
) (entity.RewardProfileAssignmentResult, error) {
	if service == nil || ctx == nil {
		return entity.RewardProfileAssignmentResult{}, invalidRewardProfileRequest(
			entity.ErrInvalidRewardProfile,
		)
	}
	command, err := entity.NormalizeRewardProfileAssignmentCommand(command)
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, invalidRewardProfileRequest(err)
	}
	var result entity.RewardProfileAssignmentResult
	err = service.journal.Execute(ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var assignmentErr error
		result, assignmentErr = service.repository.Assign(txCtx, command)
		if assignmentErr != nil {
			return platformmutation.Command{}, assignmentErr
		}
		if result.Replayed {
			return platformmutation.Command{Noop: true}, nil
		}
		profile := result.Profile
		return platformmutation.Command{
			EventID:    command.ID,
			Action:     "cultivation.reward_profile.assigned.v1",
			Resource:   "cultivation_reward_profile",
			ResourceID: profile.UserID.String(),
			Reason:     command.Reason,
			Before: map[string]any{
				"revision":         command.ExpectedRevision,
				"preview_checksum": checksumString(command.PreviewChecksum),
			},
			After:            rewardProfileDocument(profile),
			EventType:        "cultivation.reward_profile.assigned.v1",
			AggregateType:    "cultivation_reward_profile",
			AggregateID:      profile.UserID,
			AggregateVersion: profile.Revision,
			Payload:          rewardProfileDocument(profile),
		}, nil
	})
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, mapRewardProfileError(err)
	}
	return result, nil
}

func mapRewardProfileError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, entity.ErrRewardProfileNotFound):
		return apperr.New(apperr.CodeNotFound, "reward profile not found", err)
	case errors.Is(err, entity.ErrRewardProfileConflict):
		return apperr.New(
			apperr.CodeConflict,
			"reward profile changed; preview the assignment again",
			err,
		)
	case errors.Is(err, entity.ErrInvalidRewardProfile),
		errors.Is(err, entity.ErrInvalidTraitEffect):
		return invalidRewardProfileRequest(err)
	default:
		return err
	}
}

func invalidRewardProfileRequest(err error) error {
	return apperr.New(apperr.CodeBadRequest, "invalid reward profile request", err)
}

func rewardProfileDocument(profile entity.UserRewardProfileRevision) map[string]any {
	traits := make([]map[string]any, len(profile.Traits))
	for index, assignment := range profile.Traits {
		traits[index] = map[string]any{
			"trait_id":           assignment.TraitID.String(),
			"trait_code":         assignment.TraitCode,
			"trait_type":         assignment.TraitType,
			"slot":               assignment.Slot,
			"slot_order":         assignment.SlotOrder,
			"effect_revision_id": assignment.EffectRevision.ID.String(),
			"effect_revision":    assignment.EffectRevision.Revision,
			"effect_checksum":    checksumString(assignment.EffectRevision.Checksum),
		}
	}
	return map[string]any{
		"id":                  profile.ID.String(),
		"user_id":             profile.UserID.String(),
		"revision":            profile.Revision,
		"assignment_checksum": checksumString(profile.AssignmentChecksum),
		"source":              profile.Source,
		"effective_at":        profile.EffectiveAt,
		"traits":              traits,
	}
}

package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type rewardRuleService struct {
	repository ports.RewardRuleRepository
	journal    platformmutation.Executor
}

func NewRewardRuleService(
	repository ports.RewardRuleRepository,
	journal platformmutation.Executor,
) (ports.RewardRuleService, error) {
	if repository == nil || journal == nil {
		return nil, entity.ErrInvalidRewardRule
	}
	return &rewardRuleService{repository: repository, journal: journal}, nil
}

func (service *rewardRuleService) Current(ctx context.Context) (entity.RewardRuleRevision, error) {
	if service == nil || ctx == nil {
		return entity.RewardRuleRevision{}, invalidRewardRuleRequest(entity.ErrInvalidRewardRule)
	}
	rule, err := service.repository.Current(ctx, time.Now().UTC())
	return rule, mapRewardRuleError(err)
}

func (service *rewardRuleService) List(
	ctx context.Context,
	cursor *int64,
	limit int,
) (ports.RewardRulePage, error) {
	if service == nil || ctx == nil {
		return ports.RewardRulePage{}, invalidRewardRuleRequest(entity.ErrInvalidRewardRule)
	}
	if limit == 0 {
		limit = 25
	}
	page, err := service.repository.List(ctx, cursor, limit)
	return page, mapRewardRuleError(err)
}

func (service *rewardRuleService) PreviewPublication(
	ctx context.Context,
	configuration entity.RewardRuleConfiguration,
) (entity.RewardRulePublicationPreview, error) {
	if service == nil || ctx == nil {
		return entity.RewardRulePublicationPreview{}, invalidRewardRuleRequest(entity.ErrInvalidRewardRule)
	}
	current, err := service.repository.Current(ctx, time.Now().UTC())
	if err != nil {
		return entity.RewardRulePublicationPreview{}, mapRewardRuleError(err)
	}
	preview, err := entity.NewRewardRulePublicationPreview(current, configuration)
	if err != nil {
		return entity.RewardRulePublicationPreview{}, invalidRewardRuleRequest(err)
	}
	return preview, nil
}

func (service *rewardRuleService) Publish(
	ctx context.Context,
	command entity.RewardRulePublicationCommand,
) (entity.RewardRulePublicationResult, error) {
	if service == nil || ctx == nil {
		return entity.RewardRulePublicationResult{}, invalidRewardRuleRequest(entity.ErrInvalidRewardRule)
	}
	command, err := entity.NormalizeRewardRulePublicationCommand(command)
	if err != nil {
		return entity.RewardRulePublicationResult{}, invalidRewardRuleRequest(err)
	}
	var result entity.RewardRulePublicationResult
	err = service.journal.Execute(ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var currentErr error
		result, currentErr = service.repository.Publish(txCtx, command)
		if currentErr != nil {
			return platformmutation.Command{}, currentErr
		}
		if result.Replayed {
			return platformmutation.Command{Noop: true}, nil
		}
		return platformmutation.Command{
			EventID:    command.ID,
			Action:     "cultivation.reward_rule.published.v1",
			Resource:   "cultivation_reward_rule",
			ResourceID: result.Rule.ID.String(),
			Reason:     command.Reason,
			Before: map[string]any{
				"revision":         command.ExpectedRevision,
				"preview_checksum": checksumString(command.PreviewChecksum),
			},
			After: map[string]any{
				"revision":       result.Rule.Revision,
				"checksum":       checksumString(result.Rule.Checksum),
				"effective_from": result.Rule.EffectiveFrom,
			},
			EventType:        "cultivation.reward_rule.published.v1",
			AggregateType:    "cultivation_reward_rule",
			AggregateID:      uuid.NewSHA1(uuid.NameSpaceURL, []byte("judgify/cultivation/reward-rule/"+result.Rule.RuleKey)),
			AggregateVersion: result.Rule.Revision,
			Payload: map[string]any{
				"rule_key":          result.Rule.RuleKey,
				"revision":          result.Rule.Revision,
				"algorithm_version": result.Rule.AlgorithmVersion,
				"configuration":     result.Rule.Configuration,
				"checksum":          checksumString(result.Rule.Checksum),
				"effective_from":    result.Rule.EffectiveFrom,
			},
		}, nil
	})
	if err != nil {
		return entity.RewardRulePublicationResult{}, mapRewardRuleError(err)
	}
	return result, nil
}

func mapRewardRuleError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, entity.ErrInvalidRewardRule):
		return invalidRewardRuleRequest(err)
	case errors.Is(err, entity.ErrRewardRuleConflict):
		return apperr.New(apperr.CodeConflict, "reward rule changed; preview again", err)
	default:
		return err
	}
}

func invalidRewardRuleRequest(err error) error {
	return apperr.New(apperr.CodeBadRequest, "invalid reward rule request", err)
}

func checksumString(value [32]byte) string {
	const digits = "0123456789abcdef"
	encoded := make([]byte, len(value)*2)
	for index, current := range value {
		encoded[index*2] = digits[current>>4]
		encoded[index*2+1] = digits[current&15]
	}
	return string(encoded)
}

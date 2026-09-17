package http

import (
	"context"
	"encoding/hex"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type RewardRuleHandler interface {
	List(context.Context, *rewardRuleListRequest) (*rewardRulePageResponse, error)
	PreviewPublication(context.Context, *previewRewardRulePublicationRequest) (*rewardRulePublicationPreviewResponse, error)
	Publish(context.Context, *publishRewardRuleRequest) (*rewardRuleResponse, error)
}

type rewardRuleHandler struct {
	service ports.RewardRuleService
}

func NewRewardRuleHandler(service ports.RewardRuleService) RewardRuleHandler {
	return &rewardRuleHandler{service: service}
}

func (handler *rewardRuleHandler) List(
	ctx context.Context,
	request *rewardRuleListRequest,
) (*rewardRulePageResponse, error) {
	page, err := handler.service.List(ctx, request.Cursor, request.Limit)
	if err != nil {
		return nil, err
	}
	rules := make([]*rewardRuleResponse, len(page.Rules))
	for index, rule := range page.Rules {
		rules[index] = toRewardRuleResponse(rule, false)
	}
	return &rewardRulePageResponse{Rules: rules, NextCursor: page.NextCursor}, nil
}

func (handler *rewardRuleHandler) PreviewPublication(
	ctx context.Context,
	request *previewRewardRulePublicationRequest,
) (*rewardRulePublicationPreviewResponse, error) {
	preview, err := handler.service.PreviewPublication(ctx, toRewardRuleConfiguration(request.Configuration))
	if err != nil {
		return nil, err
	}
	return &rewardRulePublicationPreviewResponse{
		RuleKey: preview.RuleKey, CurrentRevision: preview.CurrentRevision,
		NextRevision:     preview.NextRevision,
		AlgorithmVersion: preview.AlgorithmVersion,
		Configuration:    toRewardRuleConfigurationResponse(preview.Configuration),
		PreviewChecksum:  hex.EncodeToString(preview.Checksum[:]),
	}, nil
}

func (handler *rewardRuleHandler) Publish(
	ctx context.Context,
	request *publishRewardRuleRequest,
) (*rewardRuleResponse, error) {
	commandID, ok := ctx.Value(cultivationCommandIDContextKey{}).(uuid.UUID)
	checksum, checksumOK := decodeCanonicalChecksum(request.PreviewChecksum)
	if !ok || commandID == uuid.Nil || !checksumOK {
		return nil, invalidCultivationCommand()
	}
	result, err := handler.service.Publish(ctx, entity.RewardRulePublicationCommand{
		ID: commandID, ExpectedRevision: request.ExpectedRevision,
		AlgorithmVersion: request.AlgorithmVersion,
		Configuration:    toRewardRuleConfiguration(request.Configuration),
		Reason:           request.Reason, PreviewChecksum: checksum,
	})
	if err != nil {
		return nil, err
	}
	return toRewardRuleResponse(result.Rule, result.Replayed), nil
}

func toRewardRuleConfiguration(value rewardRuleConfigurationRequest) entity.RewardRuleConfiguration {
	return entity.RewardRuleConfiguration{
		MultiplierBasisPoints: value.MultiplierBasisPoints,
		FlatBonus:             value.FlatBonus,
	}
}

func toRewardRuleConfigurationResponse(value entity.RewardRuleConfiguration) rewardRuleConfigurationResponse {
	return rewardRuleConfigurationResponse{
		MultiplierBasisPoints: value.MultiplierBasisPoints,
		FlatBonus:             value.FlatBonus,
	}
}

func toRewardRuleResponse(rule entity.RewardRuleRevision, replayed bool) *rewardRuleResponse {
	return &rewardRuleResponse{
		ID: rule.ID.String(), RuleKey: rule.RuleKey, Revision: rule.Revision,
		AlgorithmVersion: rule.AlgorithmVersion,
		Configuration:    toRewardRuleConfigurationResponse(rule.Configuration),
		Checksum:         hex.EncodeToString(rule.Checksum[:]),
		EffectiveFrom:    rule.EffectiveFrom, Replayed: replayed,
	}
}

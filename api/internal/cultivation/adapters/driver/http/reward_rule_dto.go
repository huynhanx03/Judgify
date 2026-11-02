package http

import "time"

type rewardRuleConfigurationRequest struct {
	MultiplierBasisPoints int64 `json:"multiplier_basis_points" validate:"min=0,max=100000"`
	FlatBonus             int64 `json:"flat_bonus" validate:"min=-1000000000,max=1000000000"`
}
type rewardRuleConfigurationResponse struct {
	MultiplierBasisPoints int64 `json:"multiplier_basis_points"`
	FlatBonus             int64 `json:"flat_bonus"`
}
type rewardRuleListRequest struct {
	Cursor *int64 `form:"cursor" validate:"omitempty,min=1"`
	Limit  int    `form:"limit" validate:"omitempty,min=1,max=100"`
}
type previewRewardRulePublicationRequest struct {
	Configuration rewardRuleConfigurationRequest `json:"configuration" validate:"required"`
}
type publishRewardRuleRequest struct {
	ExpectedRevision int64                          `json:"expected_revision" validate:"min=1"`
	AlgorithmVersion string                         `json:"algorithm_version" validate:"required,max=64"`
	Configuration    rewardRuleConfigurationRequest `json:"configuration" validate:"required"`
	Reason           string                         `json:"reason" validate:"required,min=3,max=1024"`
	PreviewChecksum  string                         `json:"preview_checksum" validate:"required,len=64"`
}
type rewardRulePublicationPreviewResponse struct {
	RuleKey          string                          `json:"rule_key"`
	CurrentRevision  int64                           `json:"current_revision"`
	NextRevision     int64                           `json:"next_revision"`
	AlgorithmVersion string                          `json:"algorithm_version"`
	Configuration    rewardRuleConfigurationResponse `json:"configuration"`
	PreviewChecksum  string                          `json:"preview_checksum"`
}
type rewardRuleResponse struct {
	ID               string                          `json:"id"`
	RuleKey          string                          `json:"rule_key"`
	Revision         int64                           `json:"revision"`
	AlgorithmVersion string                          `json:"algorithm_version"`
	Configuration    rewardRuleConfigurationResponse `json:"configuration"`
	Checksum         string                          `json:"checksum"`
	EffectiveFrom    time.Time                       `json:"effective_from"`
	Replayed         bool                            `json:"replayed,omitempty"`
}
type rewardRulePageResponse struct {
	Rules      []*rewardRuleResponse `json:"rules"`
	NextCursor *int64                `json:"next_cursor,omitempty"`
}

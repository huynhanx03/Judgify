package http

import "time"

type rewardProfileUserRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid"`
}

type rewardProfileSelectionRequest struct {
	RootTraitID    string   `json:"root_trait_id" validate:"required,uuid"`
	TalentTraitIDs []string `json:"talent_trait_ids" validate:"required,len=3,unique,dive,uuid"`
}

type previewRewardProfileAssignmentRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid"`
	rewardProfileSelectionRequest
}

type assignRewardProfileRequest struct {
	UserID           string `uri:"user_id" validate:"required,uuid"`
	ExpectedRevision int64  `json:"expected_revision" validate:"required,min=1"`
	rewardProfileSelectionRequest
	PreviewChecksum string `json:"preview_checksum" validate:"required,len=64"`
	Reason          string `json:"reason" validate:"required,min=3,max=1024"`
}

type rewardProfileTraitResponse struct {
	TraitID        string                      `json:"trait_id"`
	TraitCode      string                      `json:"trait_code"`
	TraitType      string                      `json:"trait_type"`
	TraitName      string                      `json:"trait_name"`
	Description    string                      `json:"description,omitempty"`
	RarityCode     string                      `json:"rarity_code"`
	RarityName     string                      `json:"rarity_name"`
	Slot           string                      `json:"slot"`
	SlotOrder      int                         `json:"slot_order"`
	EffectRevision traitEffectRevisionResponse `json:"effect_revision"`
}
type rewardProfileResponse struct {
	ID                 string                       `json:"id"`
	UserID             string                       `json:"user_id"`
	Revision           int64                        `json:"revision"`
	AssignmentChecksum string                       `json:"assignment_checksum"`
	Source             string                       `json:"source"`
	SourceOfferID      *string                      `json:"source_offer_id,omitempty"`
	CommandID          *string                      `json:"command_id,omitempty"`
	EffectiveAt        time.Time                    `json:"effective_at"`
	Traits             []rewardProfileTraitResponse `json:"traits"`
	Replayed           bool                         `json:"replayed,omitempty"`
}

type rewardProfileAssignmentPreviewResponse struct {
	UserID          string                       `json:"user_id"`
	CurrentRevision int64                        `json:"current_revision"`
	NextRevision    int64                        `json:"next_revision"`
	Traits          []rewardProfileTraitResponse `json:"traits"`
	PreviewChecksum string                       `json:"preview_checksum"`
}

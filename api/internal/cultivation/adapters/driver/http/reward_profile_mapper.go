package http

import (
	"fmt"

	"github.com/google/uuid"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

func toRewardProfileUserID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse reward profile user id: %w", err)
	}
	return id, nil
}
func toRewardProfileSelection(value rewardProfileSelectionRequest) (entity.RewardProfileSelection, error) {
	root, err := uuid.Parse(value.RootTraitID)
	if err != nil {
		return entity.RewardProfileSelection{}, fmt.Errorf("parse root trait id: %w", err)
	}
	talents := make([]uuid.UUID, len(value.TalentTraitIDs))
	for i, raw := range value.TalentTraitIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return entity.RewardProfileSelection{}, fmt.Errorf("parse talent trait id: %w", err)
		}
		talents[i] = id
	}
	return entity.RewardProfileSelection{RootTraitID: root, TalentTraitIDs: talents}, nil
}

package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// ToDifficultyResult converts a domain value to an application result.
func ToDifficultyResult(e *entity.Difficulty) *contract.DifficultyResult {
	if e == nil {
		return nil
	}
	return &contract.DifficultyResult{
		ID:          e.ID,
		Name:        e.Name,
		Level:       e.Level,
		ExpReward:   e.ExpReward,
		Description: e.Description,
		Version:     e.Version,
	}
}

// ToDifficultyEntityFromCreate converts a create command to a domain value.
func ToDifficultyEntityFromCreate(req *contract.CreateDifficultyCommand) *entity.Difficulty {
	return &entity.Difficulty{
		Name:        req.Name,
		Level:       req.Level,
		ExpReward:   req.ExpReward,
		Description: req.Description,
	}
}

package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// ToTagResult converts a Tag entity to its application projection.
func ToTagResult(e *entity.Tag) *contract.TagResult {
	if e == nil {
		return nil
	}
	elements := make([]*contract.TagElementResult, len(e.Elements))
	for i, el := range e.Elements {
		elements[i] = &contract.TagElementResult{ID: el.ID, Name: el.Name, Code: el.Code}
	}
	return &contract.TagResult{
		ID:       e.ID,
		Name:     e.Name,
		Elements: elements,
		Version:  e.Version,
	}
}

// ToTagEntityFromCreate converts a CreateTagCommand to a Tag entity.
func ToTagEntityFromCreate(req *contract.CreateTagCommand) *entity.Tag {
	return &entity.Tag{
		Name: req.Name,
	}
}

package usecase

import (
	"context"

	"github.com/google/uuid"

	materialcontract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
)

const (
	materialAggregate         = materialcontract.AggregateMaterial
	materialCategoryAggregate = "material_category"
)

func executeMaterialMutation(
	journal platformmutation.Executor,
	ctx context.Context,
	mutate func(context.Context) (platformmutation.Command, error),
) error {
	if journal != nil {
		return journal.Execute(ctx, mutate)
	}
	_, err := mutate(ctx)
	return err
}

func materialDocument(value *entity.Material) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	tagIDs := make([]string, len(value.Tags))
	for index := range value.Tags {
		tagIDs[index] = value.Tags[index].ID.String()
	}
	return map[string]any{
		"id": value.ID.String(), "title": value.Title,
		"description": value.Description, "content_length": len(value.Content),
		"difficulty_id": value.DifficultyID.String(), "author_id": value.AuthorID.String(),
		"category_id": value.CategoryID.String(), "status": value.Status,
		"visibility": value.Visibility, "estimated_read_time": value.EstimatedReadTime,
		"tag_ids": tagIDs,
	}
}

func materialCategoryDocument(value *entity.MaterialCategory) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id": value.ID.String(), "name": value.Name,
		"description": value.Description,
	}
}

func mutationCommand(
	action string,
	aggregate string,
	id uuid.UUID,
	reason string,
	before map[string]any,
	after map[string]any,
) platformmutation.Command {
	return platformmutation.Command{
		Action: action, Resource: aggregate, ResourceID: id.String(), Reason: reason,
		Before: before, After: after,
		EventType: action, AggregateType: aggregate, AggregateID: id,
		Payload: map[string]any{
			"resource_id": id.String(), "before": before, "after": after,
		},
	}
}

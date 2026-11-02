package usecase

import (
	"context"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
)

func executeCultivationMutation(
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

func cultivationCommand(
	action string,
	aggregate string,
	id uuid.UUID,
	version int64,
	before map[string]any,
	after map[string]any,
) platformmutation.Command {
	return platformmutation.Command{
		Action: action, Resource: aggregate, ResourceID: id.String(),
		Reason: "cultivation administration mutation",
		Before: before, After: after,
		EventType: action, AggregateType: aggregate, AggregateID: id,
		AggregateVersion: version,
		Payload: map[string]any{
			"resource_id": id.String(), "before": before, "after": after,
		},
	}
}

func elementDocument(value *entity.Element) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id": value.ID.String(), "name": value.Name, "code": value.Code,
		"description": value.Description, "version": value.Version,
	}
}

func levelDocument(value *entity.Level) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return map[string]any{"id": value.ID.String(), "name": value.Name, "min_exp": value.MinExp, "description": value.Description}
}

func rankDocument(value *entity.Rank) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return map[string]any{"id": value.ID.String(), "name": value.Name, "min_rating": value.MinRating, "description": value.Description}
}

func rarityDocument(value *entity.Rarity) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id": value.ID.String(), "name": value.Name, "code": value.Code,
		"weight": value.Weight, "description": value.Description,
	}
}

func traitDocument(value *entity.Trait) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id": value.ID.String(), "code": value.Code, "type": value.Type, "name": value.Name,
		"rarity_id": value.RarityID.String(), "description": value.Description,
		"display_order": value.DisplayOrder, "active": value.Active,
		"effect_revision": traitEffectRevisionDocument(value.EffectRevision),
		"version":         value.Version,
	}
}

// The audit document must stay JSON-shaped: the bounded encoder accepts maps
// and scalars so that its per-key redaction and node budget apply to every
// nested value, and it rejects a struct handed to it directly.
func traitEffectRevisionDocument(value *entity.TraitEffectRevision) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	elements := make([]any, 0, len(value.TargetElementIDs))
	for _, elementID := range value.TargetElementIDs {
		elements = append(elements, elementID.String())
	}
	effect := map[string]any{
		"version": value.Effect.Version,
		"kind":    string(value.Effect.Kind),
		"scope":   string(value.Effect.Scope),
	}
	if len(value.Effect.ElementCodes) > 0 {
		codes := make([]any, 0, len(value.Effect.ElementCodes))
		for _, code := range value.Effect.ElementCodes {
			codes = append(codes, code)
		}
		effect["element_codes"] = codes
	}
	if value.Effect.MultiplierDeltaBasisPoints != nil {
		effect["multiplier_delta_bps"] = *value.Effect.MultiplierDeltaBasisPoints
	}
	if value.Effect.FlatBonus != nil {
		effect["flat_bonus"] = *value.Effect.FlatBonus
	}
	return map[string]any{
		"id":                 value.ID.String(),
		"trait_id":           value.TraitID.String(),
		"revision":           value.Revision,
		"effect":             effect,
		"target_element_ids": elements,
		"effective_at":       value.EffectiveAt.UTC(),
	}
}

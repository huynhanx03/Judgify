package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate"
	"github.com/huynhanx03/judgify/internal/core/entity"
)

// BuildCreateAttributeDefinition builds the create mutation for AttributeDefinition entity.
func BuildCreateAttributeDefinition(ctx context.Context, e *entity.AttributeDefinition) *generate.AttributeDefinitionCreate {
	return global.EntClient.DB(ctx).AttributeDefinition.Create().
		SetKey(e.Key).
		SetDataType(e.DataType).
		SetDescription(e.Description)
}

// BuildUpdateAttributeDefinition builds the update mutation for AttributeDefinition entity.
func BuildUpdateAttributeDefinition(ctx context.Context, e *entity.AttributeDefinition) *generate.AttributeDefinitionUpdateOne {
	return global.EntClient.DB(ctx).AttributeDefinition.UpdateOneID(e.ID).
		SetKey(e.Key).
		SetDataType(e.DataType).
		SetDescription(e.Description)
}

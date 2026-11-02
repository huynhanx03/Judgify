package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedattribute "github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// BuildCreateAttributeDefinition builds the create mutation for AttributeDefinition entity.
func BuildCreateAttributeDefinition(client *generate.Client, e *entity.AttributeDefinition) *generate.AttributeDefinitionCreate {
	builder := client.AttributeDefinition.Create().
		SetKey(e.Key)
	if e.Kind != "" {
		builder.SetKind(generatedattribute.Kind(e.Kind))
	}
	builder.SetRequiredOnOnboarding(e.RequiredOnOnboarding)
	if e.Status != "" {
		builder.SetStatus(generatedattribute.Status(e.Status))
	}
	if e.Version > 0 {
		builder.SetVersion(e.Version)
	}
	if e.DraftRevisionID != nil {
		builder.SetDraftRevisionID(*e.DraftRevisionID)
	}
	if e.ActiveRevisionID != nil {
		builder.SetActiveRevisionID(*e.ActiveRevisionID)
	}
	return builder
}

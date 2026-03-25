package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// BuildCreateCredential builds the create mutation for Credential entity.
func BuildCreateCredential(ctx context.Context, e *entity.Credential) *generate.CredentialCreate {
	return global.EntClient.DB(ctx).Credential.Create().
		SetUserID(e.UserID).
		SetType(e.Type).
		SetCredentialData(e.CredentialData)
}

// BuildUpdateCredential builds the update mutation for Credential entity.
func BuildUpdateCredential(ctx context.Context, e *entity.Credential) *generate.CredentialUpdateOne {
	return global.EntClient.DB(ctx).Credential.UpdateOneID(e.ID).
		SetUserID(e.UserID).
		SetType(e.Type).
		SetCredentialData(e.CredentialData)
}

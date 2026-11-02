package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// BuildCreateCredential builds the create mutation for Credential entity.
func BuildCreateCredential(client *generate.Client, e *entity.Credential) *generate.CredentialCreate {
	return client.Credential.Create().
		SetUserID(e.UserID).
		SetType(e.Type).
		SetCredentialData(e.CredentialData)
}

// BuildUpdateCredential builds the update mutation for Credential entity.
func BuildUpdateCredential(client *generate.Client, e *entity.Credential) *generate.CredentialUpdateOne {
	return client.Credential.UpdateOneID(e.ID).
		SetUserID(e.UserID).
		SetType(e.Type).
		SetCredentialData(e.CredentialData)
}

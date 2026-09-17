package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// BuildCreateFederatedIdentity builds the create mutation for FederatedIdentity entity.
func BuildCreateFederatedIdentity(client *generate.Client, e *entity.FederatedIdentity) *generate.FederatedIdentityCreate {
	return client.FederatedIdentity.Create().
		SetUserID(e.UserID).
		SetProvider(e.Provider).
		SetProviderSubject(e.ProviderSubject).
		SetNillableDisplayName(e.DisplayName).
		SetNillableAvatarURL(e.AvatarURL).
		SetNillableLastAuthenticatedAt(e.LastAuthenticatedAt)
}

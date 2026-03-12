package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate"
	"github.com/huynhanx03/judgify/internal/core/entity"
)

// BuildCreateFederatedIdentity builds the create mutation for FederatedIdentity entity.
func BuildCreateFederatedIdentity(ctx context.Context, e *entity.FederatedIdentity) *generate.FederatedIdentityCreate {
	return global.EntClient.DB(ctx).FederatedIdentity.Create().
		SetUserID(e.UserID).
		SetProvider(e.Provider).
		SetExternalID(e.ExternalID)
}

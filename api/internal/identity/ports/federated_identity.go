package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// FederatedIdentityRepository defines the federated identity data access interface.
type FederatedIdentityRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.FederatedIdentity], error)
	Get(ctx context.Context, id int) (*entity.FederatedIdentity, error)
	GetByProviderAndExternalID(ctx context.Context, provider, externalID string) (*entity.FederatedIdentity, error)
	Create(ctx context.Context, e *entity.FederatedIdentity) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

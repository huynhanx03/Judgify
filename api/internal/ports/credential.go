package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/core/entity"
)

// CredentialRepository defines the credential data access interface.
type CredentialRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Credential], error)
	Get(ctx context.Context, id int) (*entity.Credential, error)
	GetByUserID(ctx context.Context, userID int, credType string) (*entity.Credential, error)
	Create(ctx context.Context, e *entity.Credential) error
	Update(ctx context.Context, e *entity.Credential) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

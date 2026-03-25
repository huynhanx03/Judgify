package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// AttributeDefinitionRepository defines the interface for attribute definition persistence.
type AttributeDefinitionRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.AttributeDefinition], error)
	FindAll(ctx context.Context) ([]*entity.AttributeDefinition, error)
	Get(ctx context.Context, id int) (*entity.AttributeDefinition, error)
	GetByKey(ctx context.Context, key string) (*entity.AttributeDefinition, error)
	Create(ctx context.Context, e *entity.AttributeDefinition) error
	Update(ctx context.Context, e *entity.AttributeDefinition) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// AttributeDefinitionService defines the attribute definition business logic interface.
type AttributeDefinitionService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.AttributeDefinitionResponse], error)
	Get(ctx context.Context, id int) (*dto.AttributeDefinitionResponse, error)
	Create(ctx context.Context, req *dto.CreateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
	Delete(ctx context.Context, id int) error
}

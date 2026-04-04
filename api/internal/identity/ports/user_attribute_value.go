package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// UserAttributeValueRepository defines the interface for user attribute value persistence.
type UserAttributeValueRepository interface {
	Get(ctx context.Context, id int) (*entity.UserAttributeValue, error)
	GetByUserID(ctx context.Context, userID int) ([]*entity.UserAttributeValue, error)
	Create(ctx context.Context, e *entity.UserAttributeValue) error
	CreateBulk(ctx context.Context, entities []*entity.UserAttributeValue) error
	Update(ctx context.Context, e *entity.UserAttributeValue) error
	UpdateBulk(ctx context.Context, entities []*entity.UserAttributeValue) error
	Delete(ctx context.Context, id int) error
	DeleteByUserID(ctx context.Context, userID int) error
}

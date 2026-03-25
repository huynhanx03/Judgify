package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	Get(ctx context.Context, id int) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	Create(ctx context.Context, e *entity.User) error
	Update(ctx context.Context, e *entity.User) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
}

// UserService defines the interface for user business logic.
type UserService interface {
	Delete(ctx context.Context, id int) error
	UpdateProfile(ctx context.Context, userID int, req *dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
	GetProfile(ctx context.Context, userID int) (*dto.ProfileResponse, error)
	GetRole(ctx context.Context, userID int) (*dto.RoleResponse, error)
}

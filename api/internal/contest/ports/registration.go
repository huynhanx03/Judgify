package ports

import (
	"context"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
)

// RegistrationRepository defines the registration data access interface.
type RegistrationRepository interface {
	Create(ctx context.Context, contestID, userID int) error
	Delete(ctx context.Context, contestID, userID int) error
	Exists(ctx context.Context, contestID, userID int) (bool, error)
	CountByContest(ctx context.Context, contestID int) (int, error)
	FindByContest(ctx context.Context, contestID int) ([]*entity.ContestRegistration, error)
}

// RegistrationService defines the registration business logic interface.
type RegistrationService interface {
	Register(ctx context.Context, contestID, userID int) error
	Unregister(ctx context.Context, contestID, userID int) error
	IsRegistered(ctx context.Context, contestID, userID int) (bool, error)
}

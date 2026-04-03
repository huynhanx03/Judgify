package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
)

// GachaService defines the gacha business logic interface.
type GachaService interface {
	LoadPool(ctx context.Context) error
	Roll(ctx context.Context) (*dto.GachaRollResponse, error)
	InvalidatePool(ctx context.Context) error
}
